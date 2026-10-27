//! Two horn presses with a complete attack and release, taken from the bus's recording.
use crate::Clip;

pub fn double_horn(source: &Clip) -> Clip { double_horn_parts(source, None, false) }

pub fn double_horn_parts(source: &Clip, release: Option<&Clip>, looping: bool) -> Clip {
    double_horn_timed(source, release, looping, 1.0).clip
}

pub struct DoubleHorn {
    pub clip: Clip,
    pub press_frames: usize,
    pub gap_frames: usize,
}

/// Apply the model's sample-rate pitch to the recording BEFORE constructing
/// the two presses. Pitching the whole sequence also shortened its silence,
/// merging the two honks perceptually on models requesting double speed.
pub fn double_horn_timed(source: &Clip, release: Option<&Clip>, looping: bool, pitch: f32) -> DoubleHorn {
    let channels = usize::from(source.channels.max(1));
    let rate = source.sample_rate.max(1) as usize;
    let frames = source.samples.len() / channels;
    let pitch = if pitch.is_finite() { pitch.clamp(0.5, 2.0) } else { 1.0 };
    let audible = |i: usize| source.samples[i*channels..(i+1)*channels].iter().any(|v| i32::from(*v).abs() > 96);
    let first = (0..frames).find(|i| audible(*i)).unwrap_or(0).saturating_sub(rate/100);
    let end = (first..frames).rfind(|i| audible(*i)).map_or(first, |i| (i+1+rate/50).min(frames));
    let available = end.saturating_sub(first);
    // Keep a short one-shot whole. A recording explicitly marked as a sustained
    // loop keeps sounding for the press, instead of becoming a tiny click when
    // its loop file is short. Both use an actual release envelope.
    let natural = (available as f32 / pitch).ceil() as usize;
    let count = if looping && available > 0 { rate*38/100 }
        else if natural <= rate*7/10 { natural }
        else { rate*38/100 };
    let mut tap = Vec::with_capacity(count*channels);
    for i in 0..count {
        let at = i as f32 * pitch;
        let a = if looping { (at as usize) % available } else { (at as usize).min(available-1) };
        let b = if looping { (a+1) % available } else { (a+1).min(available-1) };
        let f = at.fract();
        for c in 0..channels {
            let x=source.samples[(first+a)*channels+c] as f32;
            let y=source.samples[(first+b)*channels+c] as f32;
            tap.push((x+(y-x)*f) as i16);
        }
    }
    let release = release.filter(|c| available>0 && c.sample_rate == source.sample_rate && usize::from(c.channels) == channels);
    if let Some(tail) = release {
        let tail_frames = ((tail.frames() as f32 / pitch).ceil() as usize).min(rate*3/10);
        let overlap = (rate/50).min(count).min(tail_frames);
        let start = tap.len().saturating_sub(overlap*channels);
        for i in 0..tail_frames {
            let at=i as f32*pitch;
            let a=(at as usize).min(tail.frames()-1);
            let b=(a+1).min(tail.frames()-1);
            for c in 0..channels {
                let x=tail.samples[a*channels+c] as f32;
                let y=tail.samples[b*channels+c] as f32;
                let sample=x+(y-x)*at.fract();
                if i<overlap {
                    let f=i as f32/overlap.max(1) as f32;
                    tap[start+i*channels+c]=(tap[start+i*channels+c] as f32*(1.-f)+sample*f) as i16;
                } else { tap.push(sample as i16); }
            }
        }
    }
    let n = tap.len()/channels;
    for i in 0..n {
        let attack = (i as f32/(rate as f32*0.008)).min(1.);
        let ending = ((n-1-i) as f32/(rate as f32*0.055)).min(1.);
        let gain = attack.min(ending);
        let gain = gain*gain*(3.-2.*gain);
        for c in 0..channels { tap[i*channels+c] = (tap[i*channels+c] as f32*gain) as i16; }
    }
    // Quiet community recordings should be audible without clipping loud ones.
    let peak = tap.iter().map(|x| i32::from(*x).abs()).max().unwrap_or(0) as f32;
    let rms = (tap.iter().map(|v| (*v as f32).powi(2)).sum::<f32>() / tap.len().max(1) as f32).sqrt();
    let gain = if peak > 0. { (6500./rms.max(1.)).clamp(1.,2.5).min(29000./peak) } else { 1. };
    for x in &mut tap { *x = (*x as f32*gain) as i16; }
    let gap_frames = rate*24/100;
    let gap = gap_frames*channels;
    let mut samples = Vec::with_capacity(tap.len()*2+gap);
    samples.extend_from_slice(&tap); samples.resize(tap.len()+gap,0); samples.extend_from_slice(&tap);
    DoubleHorn { clip:Clip { sample_rate: source.sample_rate, channels: channels as u16, samples }, press_frames:n, gap_frames }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn complete_presses_have_release_and_silent_gap_without_clipping() {
        for rate in [22050,44100,48000] { for channels in [1,2] {
            let source = Clip { sample_rate: rate, channels, samples: vec![12000; rate as usize*channels as usize] };
            let clip = double_horn(&source);
            let tap = rate as usize*38/100*channels as usize; let gap = rate as usize*24/100*channels as usize;
            assert_eq!(clip.samples.len(),tap*2+gap);
            assert!(clip.samples[tap..tap+gap].iter().all(|v| *v==0));
            assert_eq!(&clip.samples[..tap],&clip.samples[tap+gap..]);
            assert_eq!(clip.samples[0],0); assert_eq!(*clip.samples.last().unwrap(),0);
            assert!(clip.samples.iter().all(|v| i32::from(*v).abs()<=29000));
        }}
    }
    #[test]
    fn short_recording_keeps_its_full_tail_and_loop_uses_separate_release() {
        let source = Clip { sample_rate:1000,channels:1,samples:vec![1000;300] };
        assert_eq!(double_horn(&source).frames(),840);
        let tail = Clip { sample_rate:1000,channels:1,samples:vec![-1000;100] };
        let out=double_horn_parts(&source,Some(&tail),true);
        assert!(out.samples.iter().any(|v| *v<0));
        assert_eq!(out.frames(),1160);
        let empty=Clip { sample_rate:1000,channels:1,samples:vec![] };
        assert!(double_horn(&empty).samples.iter().all(|v|*v==0));
    }

    #[test]
    fn model_pitch_cannot_shorten_the_gap_or_turn_a_short_loop_into_two_clicks() {
        for pitch in [0.5, 1.0, 2.0] {
            let source=Clip {sample_rate:48000,channels:1,samples:(0..2400)
                .map(|i|((i as f32*std::f32::consts::TAU*400.0/48000.0).sin()*12000.0) as i16).collect()};
            let out=double_horn_timed(&source,None,true,pitch);
            assert_eq!(out.press_frames,18240);
            assert_eq!(out.gap_frames,11520);
            let n=out.press_frames;let g=out.gap_frames;
            assert_eq!(&out.clip.samples[..n],&out.clip.samples[n+g..]);
            assert!(out.clip.samples[n..n+g].iter().all(|v|*v==0));
            assert!(out.clip.samples[..n].iter().filter(|v|i32::from(**v).abs()>1000).count()>n/2);
        }
    }
}
