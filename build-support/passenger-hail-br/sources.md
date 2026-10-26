# Brazilian boarding-request references

Research checked on 26 October 2026. This is a cosmetic adaptation to the
published 0.3.44 Coop15 source, not a claim that all Brazilians use one gesture.
No reference photographs or footage are redistributed in the game.

## Inspected visual evidence

- São Paulo: Quickly Travel, "サンパウロの市内バスの乗り方",
  https://ameblo.jp/quicklytravelbrasil/entry-12446637119.html
  Photograph: a passenger faces along the approaching traffic, extends the right
  arm across the front/right side towards the roadway at approximately shoulder
  height, with a nearly straight elbow, fingers together and palm down.
  The arm points away from the approaching bus's current position.
- São Luís: O Imparcial, "Estudantes poderão usar transporte de graça no dia do Enem",
  https://oimparcial.com.br/noticias/2017/11/estudantes-poderao-usar-transporte-de-graca-no-dia-do-enem/
  Photograph filename credits Thiago Veloso. Right arm held near horizontal,
  almost straight; wrist and fingers relaxed down. The person watches the
  approaching bus while the arm extends across towards the roadway.
- Rio de Janeiro: O Dia, "Mais duas linhas de ônibus voltam a circular no Rio nesta segunda",
  https://odia.ig.com.br/rio-de-janeiro/2022/08/6469699-mais-duas-linhas-de-onibus-voltam-a-circular-no-rio-nesta-segunda.html
  Photograph: arm extends outward/upward; the open fingers are spread. This
  differs from São Paulo's fingers-together example, so finger configurations
  must not be presented as a national rule.
- Agência Cidades reproduction credited to Fabio Rodrigues-Pozzebom/ABr,
  https://www.agenciacidades.com.br/reportagens/brasileiro-pega-onibus-todo-dia-percorre-135km-e-gasta-36-minutos/414/
  Photograph location is not independently confirmed; do not assign a city.
  It shows higher outward arm extension with an open hand, not a fist with
  one mechanically tracking pointer. Demonstrates variation in elevation.
- Serra, Espírito Santo: reader CCTV reproduced by Portal Tempo Novo,
  https://www.portaltemponovo.com.br/camera-flagra-momento-em-que-onibus-do-transcol-derruba-abrigo-na-serra/
  Video inspected directly, including frames 0–3.5 seconds. The passenger has
  one arm out as the bus approaches and lowers it as it draws alongside.
  The arm is partly occluded by the shelter and too small to resolve fingers.
  Do not infer an exact finger rhythm, wrist angle or universal hold time.
  The later collision is unrelated to the gesture.

A search result from official Jae also explicitly lists an index finger and an
open palm as alternatives:
https://www.facebook.com/vaidejaeoficial/posts/-o-%C3%B4nibus-t%C3%A1-vindo-voc%C3%AA-levanta-o-bra%C3%A7o-e-faz-um-sinal-para-ele-parar-qual-sinal/499146386543161/
The full post was unavailable, so it is corroborating context, not a measured
animation or evidence for choosing a population-wide distribution.

## Adaptation and evidence limits

The shared observable pattern is an extended arm, a relatively unlocked elbow
and a recognizable held boarding request, with gaze allowed to differ from
the arm direction. A single universally correct Brazilian pose is unsupported.

The implemented default uses the existing model's relaxed open-hand geometry
and extends the free arm in the posed torso's forward frame. Its existing
shoulder direction and body turn remain the reference, rather than a vector
from hand to bus. It removes the above-head wrist target and continuously
oscillating single-finger deformation. Native body motion still supplies
small natural differences between people.

The numerical choices (96% arm reach, small outward allowance, approximately
shoulder-level target and a 35-degree wrist bound) are conservative rig-fitting
choices, not angles or timings claimed to have been measured from a photograph.

Existing body/neck blending and its 27-degree relative neck yaw and 25-degree
pitch caps are retained. Route/destination eligibility, approach distance,
full-bus gating, passenger simulation, boarding, multiplayer and umbrellas
are unchanged. Roof detection and IA audio are outside this patch.

## Verification

The native Rust tests exercise the real rig and IK at several heights and
headings, both arms, real waiting poses, gaze/arm independence, disabled inputs,
finite transforms, bone lengths, hand attachment, palm orientation, shoulder
height and wrist limits. Existing hailing, neck and passenger tests run too.
A compiled preview is not equivalent to visual validation on all installed
human models or to a signed production updater release.

