#!/usr/bin/env python3
"""Generate *original* real 3D OMSI AI vehicles, not flat emergency placeholders.

Police and fire meshes are self-contained OBJ/MTL with geometrical wheels, body,
windscreens, lights, equipment, and ladders. The modified openOMSI imports OBJ.
No external models, commercial content or Python packages are required.
"""
import argparse
import math
from pathlib import Path

COLORS={
 'red':(.82,.03,.025),'white':(.91,.92,.92),'black':(.028,.030,.033),
 'rubber':(.045,.046,.050),'glass':(.085,.18,.25),'silver':(.72,.74,.75),
 'gray':(.31,.33,.34),'blue':(.02,.13,.50),'beacon':(.05,.29,.96),
 'amber':(.92,.57,.04),'brake':(.87,.045,.035),'grille':(.15,.16,.18)
}
class Mesh:
 def __init__(self): self.v=[];self.tri=[]
 def face(self,pts,material):
  n=len(self.v)+1;self.v.extend(pts)
  for i in range(1,len(pts)-1):self.tri.append((material,(n,n+i,n+i+1)))
 def box(self,x0,x1,y0,y1,z0,z1,material):
  v=[(x,y,z) for z in (z0,z1) for y in (y0,y1) for x in (x0,x1)]
  for ids in [(0,2,3,1),(4,5,7,6),(0,1,5,4),(2,6,7,3),(0,4,6,2),(1,3,7,5)]:
   self.face([v[i] for i in ids],material)
 def prism(self,base,top,mats):
  x0,x1,y0,y1,z=base;a,b,c,d,Z=top
  v=[(x0,y0,z),(x1,y0,z),(x1,y1,z),(x0,y1,z)]
  w=[(a,c,Z),(b,c,Z),(b,d,Z),(a,d,Z)]
  self.face(w,mats[0])
  for j in range(4):self.face([v[j],v[(j+1)%4],w[(j+1)%4],w[j]],mats[j+1])
 def disc(self,x,y,z,r,width,material,steps=24):
  for j in range(steps):
   a=j*2*math.pi/steps;b=(j+1)*2*math.pi/steps
   q=lambda k,h:(h,y+math.cos(k)*r,z+math.sin(k)*r)
   self.face([q(a,x),q(b,x),q(b,x+width),q(a,x+width)],material)
  for h in [x,x+width]:
   for j in range(steps):
    a=j*2*math.pi/steps;b=(j+1)*2*math.pi/steps
    self.face([(h,y,z),(h,y+math.cos(a)*r,z+math.sin(a)*r),(h,y+math.cos(b)*r,z+math.sin(b)*r)],material)
 def wheel(self,side,y,r):
  base=side*1.04 if r>.4 else side*.86
  x=base if side>0 else base-.22
  self.disc(x,y,r,r,.22,'rubber')
  self.disc(x+.23 if side>0 else x-.01,y,r,r*.60,.023,'silver')
  self.disc(x+.25 if side>0 else x-.02,y,r,r*.23,.018,'gray',18)
 def write(self,folder,name):
  folder.mkdir(parents=True,exist_ok=True)
  obj=[f'mtllib {name}.mtl',f'o {name}']
  obj.extend('v %.5f %.5f %.5f'%p for p in self.v)
  material=None
  for color,pts in self.tri:
   if material!=color:obj.append('usemtl '+color);material=color
   obj.append('f '+' '.join(map(str,pts)))
  (folder/(name+'.obj')).write_text('\n'.join(obj)+'\n',encoding='utf8')
  mtl=[]
  for color,(r,g,b) in COLORS.items():
   mtl.extend([f'newmtl {color}',f'Kd {r:.3f} {g:.3f} {b:.3f}','Ka 0.23 0.23 0.23','Ks 0.15 0.15 0.15','Ns 30'])
  (folder/(name+'.mtl')).write_text('\n'.join(mtl)+'\n',encoding='utf8')
  return len(self.tri)

def police():
 m=Mesh()
 m.box(-.85,.85,-2.3,2.3,.4,.7,'black')
 m.prism((-.94,.94,-2.32,2.25,.53),(-.84,.84,-2.1,2.12,1.02),['white']*5)
 m.prism((-.78,.78,-1.30,1.18,1.0),(-.63,.63,-.97,.78,1.68),['white','glass','glass','glass','glass'])
 m.box(-.75,.75,2.28,2.34,.55,.84,'grille')
 m.box(-.96,.96,2.24,2.38,.43,.54,'black')
 m.box(-.96,.96,-2.35,-2.24,.43,.53,'black')
 for z in (.59,.66,.73,.80):m.box(-.61,.61,2.35,2.36,z,z+.014,'silver')
 for side in (-1,1):
  a,b=(-.89,-.53) if side<0 else (.53,.89)
  m.box(a,b,2.28,2.37,.82,.94,'white')
  m.box(a,b,-2.38,-2.3,.69,.85,'brake')
  m.box(min(side*.90,side*1.06),max(side*.90,side*1.06),.68,.98,1.17,1.27,'black')
  m.box(min(side*.91,side*.94),max(side*.91,side*.94),-1.14,.82,.89,.97,'blue')
  for y in (-1.53,1.48):m.wheel(side,y,.345)
  m.box(min(side*.90,side*.93),max(side*.90,side*.93),-.24,.02,1.00,1.04,'gray')
 m.box(-.50,.50,-.27,.09,1.68,1.76,'black')
 m.box(-.47,-.02,-.24,.06,1.75,1.94,'beacon')
 m.box(.02,.47,-.24,.06,1.75,1.94,'red')
 return m

def fire():
 m=Mesh()
 m.box(-1.17,1.17,-3.55,3.4,.47,.8,'black')
 m.box(-1.14,1.14,-3.48,.76,.78,2.63,'red')
 m.prism((-1.13,1.13,.75,3.45,.80),(-1.08,1.08,.82,2.96,2.82),['red','red','red','glass','red'])
 m.box(-1.15,1.15,3.43,3.59,.55,.76,'silver')
 m.box(-1.15,1.15,-3.63,-3.44,.55,.77,'silver')
 m.box(-.94,.94,3.46,3.56,.92,1.52,'grille')
 for z in (1.01,1.13,1.25,1.37):m.box(-.78,.78,3.58,3.60,z,z+.018,'silver')
 for side in (-1,1):
  outer=1.19 if side>0 else -1.24
  lo,hi=min(outer,outer+.05),max(outer,outer+.05)
  for y0,y1 in [(-3.36,-2.08),(-2.01,-.82),(-.75,.5)]:
   m.box(lo,hi,y0,y1,1.00,2.42,'silver')
   for k in range(8):
    z=1.05+k*.164;m.box(lo-.009,hi+.009,y0+.04,y1-.04,z,z+.014,'gray')
   m.box(lo-.015,hi+.015,y0+.5,y0+.71,1.57,1.64,'black')
  m.box(lo,hi,1.2,2.3,1.95,2.19,'glass')
  m.box(min(side*1.18,side*1.34),max(side*1.18,side*1.34),2.3,2.62,1.92,2.21,'black')
  m.box(min(side*1.14,side*1.17),max(side*1.14,side*1.17),-3.35,3.3,1.23,1.29,'white')
  m.box(min(side*1.18,side*1.25),max(side*1.18,side*1.25),3.2,3.54,1.12,1.32,'white')
  m.box(min(side*1.18,side*1.25),max(side*1.18,side*1.25),-3.6,-3.32,.92,1.22,'brake')
  for y in (-2.65,-1.70,2.0):m.wheel(side,y,.49)
 for x in (-.68,.68):m.box(x-.05,x+.05,-3.23,.51,2.86,2.99,'silver')
 for k in range(16):
  y=-3.15+k*.227;m.box(-.68,.68,y,y+.055,2.97,3.035,'silver')
 for x in (-.8,.8):
  m.box(x-.18,x+.18,2.40,2.79,2.81,2.93,'black')
  m.box(x-.15,x+.15,2.43,2.77,2.92,3.17,'beacon')
 m.box(-.25,.25,2.48,2.73,2.88,3.03,'silver')
 for x in (-.63,.63):m.disc(x,-3.6,1.60,.30,.09,'gray',24)
 m.box(-.95,.95,-3.66,-3.49,.87,.97,'amber')
 return m

def emit(root,name,mesh,width,length,height,mass,diameter):
 folder=root/'Vehicles'/'OpenOMSI_BCS'/name
 tris=mesh.write(folder/'model',name)
 assert tris>900,(name,tris)
 (folder/'model'/'model.cfg').write_text(f'[mesh]\n{name}.obj\n',encoding='utf8')
 (folder/(name+'.ovh')).write_text(
  f'[friendlyname]\nOpenOMSI BCS\n{name} Emergency Vehicle\n{name} Emergency Vehicle\n'
  f'\n[type]\n0\n\n[model]\nmodel\\model.cfg\n'
  f'\n[boundingbox]\n{width}\n{length}\n{height}\n0\n0\n{height/2}\n'
  f'\n[mass]\n{mass}\n'
  f'\n[newachse]\n[achse_long]\n{length*.29:.2f}\n[achse_maxwidth]\n{width}\n'
  f'[achse_minwidth]\n{width*.65:.2f}\n[achse_raddurchmesser]\n{diameter}\n'
  f'\n[newachse]\n[achse_long]\n{-length*.31:.2f}\n[achse_maxwidth]\n{width}\n'
  f'[achse_minwidth]\n{width*.65:.2f}\n[achse_raddurchmesser]\n{diameter}\n',encoding='utf8')
 print(name,':',tris,'3D triangles')

def generate(root):
 emit(root,'Police',police(),2.3,4.9,2.0,1.6,.69)
 emit(root,'Fire',fire(),2.9,7.8,3.3,12.0,.98)
 (root/'LICENSE-emergency-fallbacks.txt').write_text(
  'Original, AI-generated 3D meshes for modified openOMSI, dedicated under CC0-1.0.\n'
  'No third-party or commercial OMSI model is included.\n',encoding='utf8')

if __name__=='__main__':
 p=argparse.ArgumentParser()
 p.add_argument('--root',required=True,type=Path)
 generate(p.parse_args().root)
