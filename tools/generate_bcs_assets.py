#!/usr/bin/env python3
"""Original low-poly emergency fallback assets, generated without vendor content.
Geometry, scripts and synthesized alert audio are CC0-1.0 (OpenOMSI BCS Bridge, 2026).
"""
import argparse,math,struct,wave
from pathlib import Path
MATERIALS={'red':(.72,.025,.035),'white':(.9,.92,.92),'blue':(.025,.17,.8),'black':(.025,.03,.035),'glass':(.06,.15,.22),'metal':(.44,.49,.52),'lamp':(1,.95,.7)}
class Model:
 def __init__(self):self.v=[];self.faces=[]
 def quad(self,pts,mat):
  start=len(self.v)+1;self.v.extend(pts);self.faces.append((mat,[start+i for i in [0,1,2,3]]))
 def box(self,x,y,z,w,l,h,mat):
  a,b=x-w/2,x+w/2;c,d=y-l/2,y+l/2;e,f=z-h/2,z+h/2
  for q in [[(a,c,e),(a,c,f),(a,d,f),(a,d,e)],[(b,c,e),(b,d,e),(b,d,f),(b,c,f)],[(a,c,e),(b,c,e),(b,c,f),(a,c,f)],[(a,d,e),(a,d,f),(b,d,f),(b,d,e)],[(a,c,e),(a,d,e),(b,d,e),(b,c,e)],[(a,c,f),(b,c,f),(b,d,f),(a,d,f)]]:self.quad(q,mat)
 def wheel(self,x,y,z,r):
  for k in range(24):
   a,b=k*math.tau/24,(k+1)*math.tau/24
   self.quad([(x-.12,y+math.sin(a)*r,z+math.cos(a)*r),(x+.12,y+math.sin(a)*r,z+math.cos(a)*r),(x+.12,y+math.sin(b)*r,z+math.cos(b)*r),(x-.12,y+math.sin(b)*r,z+math.cos(b)*r)],'black')
  for side in [-1,1]:
   points=[(x+side*.121,y+math.sin(k*math.tau/24)*r*.62,z+math.cos(k*math.tau/24)*r*.62)for k in range(24)]
   start=len(self.v)+1;self.v.extend(points);self.faces.append(('metal',[start+k for k in (range(24)if side>0 else reversed(range(24)))]))
 def save(self,path):
  # OBJ loader interprets Y as up and Z as the length of the vehicle;
  # the cabin generator stores Y forward and Z up. Export the inverse
  # mapping, otherwise the importer turns a 4.7 m car into a 1.8 m one.
  out=['# Original CC0 emergency fallback geometry','mtllib materials.mtl'];out +=['v %.5f %.5f %.5f'%(v[0],v[2],v[1]) for v in self.v]
  current=None
  for mat,face in self.faces:
   if mat!=current:out.append('usemtl '+mat);current=mat
   # Fan triangles work in both the native OBJ reader and modelling tools.
   for k in range(1,len(face)-1):out.append('f %d %d %d'%(face[0],face[k],face[k+1]))
  path.write_text('\n'.join(out)+'\n')
def generate(root):
 for kind in ['Police','Fire']:
  folder=root/'Vehicles/OpenOMSI_BCS'/kind;folder.mkdir(parents=True,exist_ok=True)
  body=Model();lights=Model();truck=kind=='Fire';width,length,height=(2.4,7.4,3.15)if truck else(1.85,4.7,1.65)
  if truck:
   body.box(0,0,.64,2.1,7.2,.35,'black');body.box(0,-1.1,1.85,2.35,4.5,2.25,'red');body.box(0,2.37,1.48,2.35,2.2,1.9,'red')
   body.box(0,3.485,1.98,2.08,.02,.75,'glass');body.box(0,2.38,2.43,2.4,2.25,.14,'white');body.box(0,3.53,.72,2.4,.18,.2,'metal')
   for side in [-1,1]:
    body.box(side*1.19,2.35,2.0,.025,1.48,.7,'glass');body.box(side*1.19,-1.1,1.83,.025,4.05,1.6,'metal')
    for y in [-2.45,-1.1,.25]:body.box(side*1.205,y,1.83,.025,.045,1.6,'white')
    body.box(side*1.21,0,1.02,.025,6.6,.18,'white')
    for y in [-2.3,2.5]:body.wheel(side*1.12,y,.5,.49)
    body.box(side*.54,-.9,3.04,.07,5.3,.07,'metal')
   for y in range(-3,2):body.box(0,y,3.05,1.15,.055,.055,'metal')
   for side in [-1,1]:lights.box(side*.72,2.4,2.64,.4,.4,.28,'blue')
  else:
   body.box(0,0,.64,1.8,4.6,.7,'white');body.box(0,-.1,1.15,1.55,2.4,.7,'glass');body.box(0,-.1,1.55,1.65,2.45,.12,'white')
   body.box(0,1.65,.98,1.77,1.3,.12,'black');body.box(0,-1.9,.98,1.77,.75,.12,'black')
   for side in [-1,1]:
    body.box(side*.91,0,.85,.02,4.3,.2,'blue')
    for y in [-1.45,1.5]:body.wheel(side*.83,y,.36,.36)
   lights.box(-.46,0,1.73,.44,.32,.18,'blue');lights.box(.46,0,1.73,.44,.32,.18,'blue')
  for side in [-1,1]:body.box(side*width*.35,length/2+.01,.97 if truck else .73,.3,.03,.17,'lamp')
  body.save(folder/'body.obj');lights.save(folder/'beacons.obj')
  mtl=''
  for name,rgb in MATERIALS.items():
   mtl+=f'newmtl {name}\nKd {rgb[0]} {rgb[1]} {rgb[2]}\nKa .08 .08 .08\nKs .15 .15 .15\nNs 20\n'
   if name in ['blue','lamp']:mtl+='Ke .1 .2 .65\n'if name=='blue'else'Ke .3 .25 .1\n'
  (folder/'materials.mtl').write_text(mtl)
  (folder/'model.cfg').write_text('[mesh]\nbody.obj\n\n[mesh]\nbeacons.obj\n\n[visible]\nbcs_blue\n1\n')
  (folder/(kind+'.ovh')).write_text(f'[friendlyname]\nOpenOMSI BCS\n{kind} responder\nGeneric CC0 fallback\n\n[ai_veh_type]\n0\n[model]\nmodel.cfg\n[sound]\nsound.cfg\n[mass]\n{9 if truck else 1.6}\n[boundingbox]\n{width}\n{length}\n{height}\n0\n0\n{height/2}\n[varnamelist]\n1\nvars.txt\n[script]\n1\nresponse.osc\n')
  (folder/'vars.txt').write_text('bcs_siren\nbcs_beacon\nbcs_blue\nbcs_pitch\n')
  (folder/'response.osc').write_text('{init}\n1 (S.L.bcs_pitch)\n0 (S.L.bcs_siren) (S.L.bcs_beacon)\n{end}\n{frame}\n(L.L.bcs_beacon) 0.5 >\n{if}\n(L.S.Time) 9 * sin 0 > (S.L.bcs_blue)\n{else}\n0 (S.L.bcs_blue)\n{endif}\n{end}\n')
  (folder/'sound.cfg').write_text('[loopsound]\nsiren.wav\n22050\nbcs_pitch\n1\n0.6\n[volcurve]\nbcs_siren\n[pnt]\n0\n0\n[pnt]\n1\n1\n[3d]\n0\n0\n1.5\n100\n')
  rate=22050
  with wave.open(str(folder/'siren.wav'),'wb')as f:
   f.setparams((1,2,rate,0,'NONE','not compressed'))
   f.writeframes(b''.join(struct.pack('<h',int(11000*math.sin(math.tau*(660 if int(i/rate*2)%2==0 else 880)*i/rate)))for i in range(rate*2)))
 (root/'LICENSE-emergency-fallbacks.txt').write_text('Original fictional generic Police and Fire vehicle assets, including geometry, scripts and synthesized alert audio.\nOpenOMSI BCS Bridge contributors, 2026. Dedicated to the public domain under CC0 1.0 Universal.\nNo OEM, third-party game content or trademark assets are included.\nGenerator source accompanies the release. https://creativecommons.org/publicdomain/zero/1.0/\n')
if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('output',type=Path);generate(p.parse_args().output)
