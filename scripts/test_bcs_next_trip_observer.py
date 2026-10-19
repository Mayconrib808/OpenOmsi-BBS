#!/usr/bin/env python3
"""Verify the observer hotfix in a real JVM, using only synthetic BCS APIs.

An optional local --jar also checks the user's exact logger without executing
the vendor application, loading its state classes, or redistributing its code.
"""
import argparse
import base64
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
JAVA = shutil.which('java') or '/usr/bin/java'
LEGACY_LOGGER = r'''package bbs;
import java.util.*;
public class f {
 private List<String> d = new ArrayList<String>();
 public f(String name) {}
 public void a(String text) { d.add(bridgeNextTripDetails(text)); }
 private static String bridgeNextTripDetails(String text) {
  if (text == null || !(text.endsWith("Weitere Tour Multiplayer") ||
      text.endsWith("Weitere Event Tour Multiplayer"))) return text;
  try {
   if (!system.m.al() || system.I.h() != 2 || system.I.I() <= 0 ||
       system.I.i() == null || system.I.j() == null) return text;
   return text + "\n[OpenOmsi-BBS next-trip log v1]\nSchicht ID: " +
       system.I.I() + "\nKarte: " + system.I.i().e() +
       "\nTour: " + system.I.j().c();
  } catch (Throwable unavailable) { return text; }
 }
}'''
STATE = '''package system; public class CLASS {
 public static byte mode=2; public static long shift=4999704;
 public static r.n map=new r.n(); public static r.E trip=new r.E();
 public static byte h(){return mode;} public static long I(){return shift;}
 public static r.n i(){return map;} public static r.E j(){return trip;}
}'''
CHECK = r'''import java.lang.reflect.*; import java.util.*;
public class ObserverRuntimeCheck {
 public static void main(String[] args) throws Exception {
  Class<?> logger=Class.forName(args[0]), state=Class.forName(args[1]);
  Method observer=logger.getDeclaredMethod("bridgeNextTripDetails",String.class);
  observer.setAccessible(true);
  String event="---\nWeitere Event Tour Multiplayer";
  if(args.length>2) {
   if(!event.equals(observer.invoke(null,event))) throw new AssertionError("baseline did not reproduce");
   System.out.println("PASS: old observer reproduces the missing continuation with the damage flag off"); return;
  }
  for(String text:new String[]{event,"Weitere Tour Multiplayer"}) {
   String result=(String)observer.invoke(null,text);
   if(!result.contains("Schicht ID: 4999704") || !result.contains("Karte: Test") ||
      !result.contains("Tour: 19:30:00") || !result.contains("(Linie: 159)"))
       throw new AssertionError(result);
  }
  for(String text:new String[]{"OpenOMSI hat geladen","Schicht abschließen: 4999703",event+" pending"})
   if(!text.equals(observer.invoke(null,text))) throw new AssertionError("unrelated log changed");
  if(observer.invoke(null,new Object[]{null})!=null) throw new AssertionError("null");
  for(byte mode:new byte[]{0,1,3}) {
   state.getField("mode").setByte(null,mode);
   if(!event.equals(observer.invoke(null,event))) throw new AssertionError("non-multiplayer mode");
  }
  state.getField("mode").setByte(null,(byte)2);
  for(long shift:new long[]{0,-1}) {
   state.getField("shift").setLong(null,shift);
   if(!event.equals(observer.invoke(null,event))) throw new AssertionError("invalid shift");
  }
  state.getField("shift").setLong(null,4999704);
  for(String field:new String[]{"map","trip"}) {
   Field f=state.getField(field); Object value=f.get(null); f.set(null,null);
   if(!event.equals(observer.invoke(null,event))) throw new AssertionError("incomplete selection");
   f.set(null,value);
  }
  if(args[0].equals("bbs.f")) {
   Object instance=logger.getConstructor(String.class).newInstance("isolated-fixture.txt");
   logger.getMethod("a",String.class).invoke(instance,event);
   Field list=logger.getDeclaredField("d");list.setAccessible(true);
   if(!list.get(instance).toString().contains("Schicht ID: 4999704"))
     throw new AssertionError("actual logger entry did not expose selection");
  }
  System.out.println("PASS: JVM verification, normal/event continuations, unrelated logs and incomplete selections: "+args[0]+" / "+args[1]);
 }
}'''
HARNESS = '''package main
import ("encoding/json"; "fmt"; "os"; "path/filepath"; "strings")
type Config struct { Root, Language string }
func samePath(a,b string) bool { return strings.EqualFold(filepath.Clean(a),filepath.Clean(b)) }
func localText(lang,pt,en,de string) string { return pt }
func main() {
 var value interface{}; var err error
 if os.Args[1]=="owner" { value,err=bcsTripStateOwner(os.Args[2])
 } else { value,err=prepareBCSNextTrip(Config{Root:os.Args[2],Language:"en"},nil) }
 if err!=nil { panic(err) }; b,_:=json.Marshal(value);fmt.Println(string(b))
}'''


def run(*args, **kwargs):
    return subprocess.run(args, check=True, **kwargs)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--jar', type=Path)
    p.add_argument('--write-fixture', type=Path)
    args = p.parse_args()
    with tempfile.TemporaryDirectory(prefix='bcs-observer-regression-') as tmp:
        work = Path(tmp)
        source, classes = work/'java', work/'classes'
        files = {
            'r/n.java': 'package r; public class n { public String e(){return "Test";} }',
            'r/E.java': 'package r; public class E { public String c(){return "19:30:00 - 19:50:00 (Linie: 159)\\nA - B (Umlauf: 1 (Sw))";} }',
            'system/m.java': 'package system; public class m { public static boolean al(){return false;} }',
            'system/I.java': STATE.replace('CLASS', 'I'),
            'pedepe_helper/a.java': 'package pedepe_helper; public class a { public static String a(String s){return s;} }',
            'pedepe_helper/d.java': 'package pedepe_helper; import java.util.*; public class d { public static boolean c(String s){return false;} public static List<String> a(String a,String b){return new ArrayList<String>();} public static void a(List<String> l,String p,String e){} }',
            'pedepe_helper/n.java': 'package pedepe_helper; public class n { public static String e(long t){return "19.10.2026 03:23:45:000";} }',
            'ObserverRuntimeCheck.java': CHECK,
        }
        go = work/'go'; go.mkdir()
        for name in ('bcs_trip_auto.go', 'bcs_trip_auto_payload.go', 'bcs_trip_log_patch.go'):
            shutil.copy2(ROOT/'source'/name, go/name)
        (go/'harness.go').write_text(HARNESS)
        helper = work/('observer-repair.exe' if os.name == 'nt' else 'observer-repair')
        env = dict(os.environ, GO111MODULE='off', GOTOOLCHAIN='local', CGO_ENABLED='0')
        run(shutil.which('go'), 'build', '-o', str(helper), *map(str, go.glob('*.go')), env=env)

        def helper_json(*parts):
            return json.loads(subprocess.check_output([str(helper), *map(str, parts)], text=True))

        if args.jar:
            owner = helper_json('owner', args.jar.resolve())
            assert owner.startswith('system/') and owner[7:].isalpha(), owner
            files[owner+'.java'] = STATE.replace('CLASS', owner[7:])
        for name, text in files.items():
            dest = source/name; dest.parent.mkdir(parents=True, exist_ok=True); dest.write_text(text)
        classes.mkdir()
        run(JAVA, '-m', 'jdk.compiler/com.sun.tools.javac.Main', '--release', '8', '-d', str(classes),
            *map(str, source.rglob('*.java')), str(ROOT/'scripts/bcs_next_trip/ObserverTemplate.java'))
        legacy = source/'bbs/f.java'; legacy.parent.mkdir(); legacy.write_text(LEGACY_LOGGER)
        legacy_classes = work/'legacy-classes'; legacy_classes.mkdir()
        run(JAVA, '-m', 'jdk.compiler/com.sun.tools.javac.Main', '--release', '8', '-cp', str(classes),
            '-d', str(legacy_classes), str(legacy))
        fixture = (legacy_classes/'bbs/f.class').read_bytes()
        if args.write_fixture:
            args.write_fixture.write_text(base64.b64encode(fixture).decode()+'\n')

        def check(logger_dir, state, baseline=False, name='bbs.f'):
            run(JAVA, '-Xverify:all', '-cp', os.pathsep.join(map(str, (classes, logger_dir))),
                'ObserverRuntimeCheck', name, state, *(['baseline'] if baseline else []))

        check(legacy_classes, 'system.I', True)
        check(legacy_classes, 'system.I', name='bridge.ObserverTemplate')
        cases = [('synthetic', None, 'system/I')]
        if args.jar:
            cases.append(('local-vendor', args.jar.resolve(), owner))
        for label, original, state in cases:
            case = work/label; case.mkdir()
            jar = case/'busbetrieb-simulator.jar'
            if original:
                shutil.copy2(original, jar)
            else:
                with zipfile.ZipFile(jar, 'w') as z:
                    z.writestr('bbs/f.class', fixture)
                    z.write(classes/'system/I.class', 'system/I.class')
                    z.writestr('unchanged.txt', 'settings, telemetry and authentication stay unchanged')
            before = jar.read_bytes()
            with zipfile.ZipFile(jar) as z:
                entries = {n: z.read(n) for n in z.namelist()}
            result = helper_json('prepare', case)
            assert result['Changed'] and not result['Note'], result
            assert Path(str(jar)+'.antes-proxima-viagem.bak').read_bytes() == before
            with zipfile.ZipFile(jar) as z:
                assert set(z.namelist()) == set(entries)
                for n, data in entries.items():
                    if n != 'bbs/f.class':
                        assert z.read(n) == data, n
                logger = work/(label+'-logger')/'bbs/f.class'
                logger.parent.mkdir(parents=True); logger.write_bytes(z.read('bbs/f.class'))
            check(logger.parent.parent, state.replace('/', '.'))
            after = jar.read_bytes()
            assert not helper_json('prepare', case)['Changed']
            assert jar.read_bytes() == after
            print('PASS:', label, 'archive backup, every non-observer entry and repeat installation')


if __name__ == '__main__':
    main()
