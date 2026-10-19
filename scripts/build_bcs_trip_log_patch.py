#!/usr/bin/env python3
"""Build the narrowly scoped logger update from a locally supplied BCS JAR.

No vendor JAR or vendor class is committed to the repository. This tool checks
the inspected JAR hash, compiles the observational method, and embeds the one
modified logger class into the optional Windows updater.
"""
import argparse
import base64
import difflib
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import struct
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
JAVA = shutil.which('java') or '/usr/bin/java'
ASM_EXPORTS = [
    '--add-exports=java.base/jdk.internal.org.objectweb.asm=ALL-UNNAMED',
    '--add-exports=java.base/jdk.internal.org.objectweb.asm.tree=ALL-UNNAMED',
]


def run(*args, **kwargs):
    subprocess.run(args, check=True, **kwargs)


def emit_automatic_delta(original, patched, destination, jar_sha):
    delta = bytearray(b'BCSDELTA1')
    cursor = 0
    for block in difflib.SequenceMatcher(None, original, patched, autojunk=False).get_matching_blocks():
        if block.b > cursor:
            literal = patched[cursor:block.b]
            delta.extend(b'\x01' + struct.pack('>I', len(literal)) + literal)
        if block.size:
            delta.extend(b'\x00' + struct.pack('>II', block.a, block.size))
        cursor = block.b + block.size
    payload = base64.b64encode(delta).decode('ascii')
    lines = '\n'.join('\t"'+payload[i:i+100]+'" +' for i in range(0,len(payload),100)).rstrip(' +')
    destination.write_text(
        'package main\n\n// Generated from ObserverTemplate.java and ApplyObserver.java.\n'
        f'const bcsInspectedJarSHA = "{jar_sha}"\n'
        f'const bcsOriginalLoggerSHA = "{hashlib.sha256(original).hexdigest()}"\n'
        f'const bcsAutoLoggerSHA = "{hashlib.sha256(patched).hexdigest()}"\n'
        'const bcsPreviousLoggerSHA = "7c5ab872e7e846433fa9045a68fde198e2f39a45efadd948d19b43d977b9cb13"\n'
        'const bcsAutoLoggerDelta =\n'+lines+'\n')


def main():
    p = argparse.ArgumentParser()
    p.add_argument('jar', type=Path)
    p.add_argument('--go', default='go')
    p.add_argument('--output', type=Path, required=True)
    p.add_argument('--keep-test-dir', type=Path)
    p.add_argument('--emit-auto-delta', type=Path, help='Regenerate the source-only automatic Setup delta.')
    args = p.parse_args()
    args.output = args.output.resolve()
    args.jar = args.jar.resolve()
    inspected_hash = hashlib.sha256(args.jar.read_bytes()).hexdigest()
    inspected_builds = {
        (ROOT/'source/bcs_trip_log_patch.sha256').read_text().strip(): 'I',
        'bfbf51cbb9033b130ecd4bebd41e1d04b926d8ef6c578737f1363a3146b196c6': 'I',
        '22d01c3f84ef48266293c49aca082c9792855c00379ae28e5318b11e1ed34555': 'J',
    }
    if inspected_hash not in inspected_builds:
        raise SystemExit('The supplied JAR is not the exact inspected BCS build.')
    work = args.keep_test_dir or Path(tempfile.mkdtemp(prefix='bbs-log-observer-'))
    work.mkdir(parents=True, exist_ok=True)
    fixtures = {
        'r/n.java': 'package r; public class n { public String e(){return "SP_Carrao City";} }',
        'r/E.java': 'package r; public class E { public String c(){return "21:10:00 - 22:10:00 (Linie: 3539)\\nCidade Tiradentes 3539 - Metro Bresser 3539 (Umlauf: 02)";} }',
        'system/m.java': 'package system; public class m { public static boolean nativeMode=true; public static boolean al(){return nativeMode;} }',
        'system/I.java': 'package system; public class I { public static long shift=4951233; public static r.E trip=new r.E(); public static byte h(){return 2;} public static long I(){return shift;} public static r.n i(){return new r.n();} public static r.E j(){return trip;} }',
        'pedepe_helper/a.java': 'package pedepe_helper; public class a { public static String a(String s){return s;} }',
        'pedepe_helper/d.java': 'package pedepe_helper; import java.util.*; public class d { public static boolean c(String s){return false;} public static List<String> a(String a,String b){return new ArrayList<>();} public static void a(List<String> l,String p,String e){} }',
        'pedepe_helper/n.java': 'package pedepe_helper; public class n { public static String e(long t){return "11.10.2026 05:00:00:000";} }',
        'ObserverCheck.java': '''import java.lang.reflect.*;
public class ObserverCheck {
 public static void main(String[] args) throws Exception {
  Class<?> c=Class.forName("bbs.f");
  Method observer=c.getDeclaredMethod("bridgeNextTripDetails",String.class); observer.setAccessible(true);
  String event="---\\nWeitere Event Tour Multiplayer";
  String result=(String)observer.invoke(null,event);
  if(!result.contains("Schicht ID: 4951233") || !result.contains("Tour: 21:10:00") || !result.contains("Cidade Tiradentes 3539 - Metro Bresser 3539 (Umlauf: 02)")) throw new AssertionError(result);
  if(!((String)observer.invoke(null,"Weitere Tour Multiplayer")).contains("Schicht ID: 4951233")) throw new AssertionError("normal continuation");
  for(String s:new String[]{"OpenOMSI hat geladen","Schicht abschließen: 4951232","Weitere Event Tour Multiplayer pending"})
   if(!s.equals(observer.invoke(null,s))) throw new AssertionError("unrelated log changed");
  if(observer.invoke(null,new Object[]{null})!=null) throw new AssertionError("null");
  system.m.nativeMode=false;
  if(!((String)observer.invoke(null,event)).contains("Schicht ID: 4951233")) throw new AssertionError("continuation depends on an unrelated vehicle flag");
  system.m.nativeMode=true; system.I.trip=null;
  if(!event.equals(observer.invoke(null,event))) throw new AssertionError("incomplete selection");
  system.I.trip=new r.E(); system.I.shift=-1;
  if(!event.equals(observer.invoke(null,event))) throw new AssertionError("invalid shift");
  system.I.shift=4951233;
  Object logger=c.getConstructor(String.class).newInstance("isolated-fixture.txt");
  c.getMethod("a",String.class).invoke(logger,event);
  Field list=c.getDeclaredField("d"); list.setAccessible(true);
  if(!list.get(logger).toString().contains("Schicht ID: 4951233")) throw new AssertionError("actual logger entry did not enrich");
  System.out.println("PASS: actual patched logger verifies; accepted-trip metadata and fallback behavior");
  System.out.println(result);
 }
}''',
    }
    state_class = inspected_builds[inspected_hash]
    if state_class != 'I':
        fixtures = {n.replace('system/I.java', 'system/'+state_class+'.java'):
                    t.replace('system.I', 'system.'+state_class).replace('public class I {', 'public class '+state_class+' {')
                    for n, t in fixtures.items()}
    for n, t in fixtures.items():
        path=work/n; path.parent.mkdir(parents=True,exist_ok=True); path.write_text(t)
    classes=work/'classes'; classes.mkdir(exist_ok=True)
    template=ROOT/'scripts/bcs_next_trip/ObserverTemplate.java'
    if state_class != 'I':
        local_template=work/'ObserverTemplate.java'
        local_template.write_text(template.read_text().replace('system.I', 'system.'+state_class))
        template=local_template
    run(JAVA,'-m','jdk.compiler/com.sun.tools.javac.Main','--release','8','-d',str(classes),
        *[str(work/n) for n in fixtures],str(template))
    instrumenter=ROOT/'scripts/bcs_next_trip/ApplyObserver.java'
    run(JAVA,'-m','jdk.compiler/com.sun.tools.javac.Main',*ASM_EXPORTS,'-d',str(classes),str(instrumenter))
    with zipfile.ZipFile(args.jar) as jar:
        if any(n.startswith('META-INF/') and n.upper().endswith(('.SF','.RSA','.DSA','.EC')) for n in jar.namelist()):
            raise SystemExit('Signed vendor archives must not be patched.')
        original=work/'original-logger.class'; original.write_bytes(jar.read('bbs/f.class'))
    patched=ROOT/'source/bcs_trip_log_patch.bin'
    run(JAVA,*ASM_EXPORTS,'-cp',str(classes),'ApplyObserver',str(original),
        str(classes/'bridge/ObserverTemplate.class'),str(patched))
    # V2 intentionally differs from the historical v1 payload. Verify the
    # generated helper in the JVM below; the input JAR remains hash-pinned.
    target=classes/'bbs/f.class'; target.parent.mkdir(exist_ok=True); target.write_bytes(patched.read_bytes())
    result=subprocess.run([JAVA,'-Xverify:all','-cp',str(classes),'ObserverCheck'],
        check=True,capture_output=True,text=True)
    print(result.stdout)
    (work/'accepted-continuation.txt').write_text(result.stdout.split('\n',1)[1])
    if args.emit_auto_delta:
        emit_automatic_delta(original.read_bytes(),patched.read_bytes(),args.emit_auto_delta,inspected_hash)
    args.output.parent.mkdir(parents=True,exist_ok=True)
    env=os.environ.copy(); env.update(GOOS='windows',GOARCH='386',GO386='sse2',CGO_ENABLED='0')
    try:
        run(args.go,'build','-trimpath','-buildvcs=false','-ldflags=-s -w -H=windowsgui',
            '-o',str(args.output),str(ROOT/'source/bcs_trip_log_patch_main.go'),
            str(ROOT/'source/bcs_trip_log_patch.go'),env=env,cwd=ROOT/'source')
    finally:
        patched.unlink(missing_ok=True)
    print('BUILT',args.output,args.output.stat().st_size)


if __name__ == '__main__':
    main()
