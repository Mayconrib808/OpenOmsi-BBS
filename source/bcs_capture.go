package main

import (
 "archive/zip"
 "bytes"
 "encoding/base64"
 "encoding/binary"
 "fmt"
 "io"
 "strings"
)

// Wrap only our existing logging extension. All vendor methods and their code,
// offsets, exception tables and stack maps remain byte-for-byte unchanged.
// Startup is lazy on the first log entry, works with the normal BCS launcher,
// and never changes global Java options or the vendor's main class.
func bcsCaptureLogger(data []byte)([]byte,error){
 methods,ok:=bcsClassMethods(data)
 if !ok||!methods["bridgeNextTripDetails(Ljava/lang/String;)Ljava/lang/String;"] {return nil,errBCSObserverUnavailable}
 const wrapper="bridgeCaptureAndNextTripDetails"
 if methods[wrapper+"(Ljava/lang/String;)Ljava/lang/String;"] {return data,nil}
 type cpEntry struct{tag byte; at int; a,b uint16; text string}
 u2:=func(at int)uint16{return binary.BigEndian.Uint16(data[at:])}
 count:=int(u2(8));pool:=make([]cpEntry,count);p:=10
 for i:=1;i<count;i++{
  if p>=len(data){return nil,errBCSObserverUnavailable}
  e:=cpEntry{tag:data[p],at:p};p++
  size:=0
  switch e.tag{
  case 1:
   if p+2>len(data){return nil,errBCSObserverUnavailable};n:=int(u2(p));p+=2
   if p+n>len(data){return nil,errBCSObserverUnavailable};e.text=string(data[p:p+n]);size=n
  case 3,4,9,10,11,12,17,18:size=4
  case 5,6:size=8;i++
  case 7,8,16,19,20:size=2
  case 15:size=3
  default:return nil,errBCSObserverUnavailable
  }
  if p+size>len(data){return nil,errBCSObserverUnavailable}
  if e.tag==7||e.tag==10||e.tag==12 {e.a=u2(p);if size==4{e.b=u2(p+2)}}
  pool[i]=e;p+=size
 }
 cpEnd:=p
 if p+8>len(data){return nil,errBCSObserverUnavailable}
 owner:=u2(p+2)
 if int(owner)>=count||pool[owner].tag!=7||int(pool[owner].a)>=count||pool[pool[owner].a].text!="bbs/f" {return nil,errBCSObserverUnavailable}
 name,desc,call:=uint16(0),uint16(0),0
 for _,e:=range pool {
  if e.tag!=10||e.a!=owner||int(e.b)>=count{continue}
  nt:=pool[e.b]
  if nt.tag!=12||int(nt.a)>=count||int(nt.b)>=count{continue}
  if pool[nt.a].text=="bridgeNextTripDetails"&&pool[nt.b].text=="(Ljava/lang/String;)Ljava/lang/String;"{
   if call!=0{return nil,errBCSObserverUnavailable};name=nt.a;desc=nt.b;call=e.at
  }
 }
 if call==0{return nil,errBCSObserverUnavailable}
 p+=6
 interfaces:=int(u2(p));p+=2+interfaces*2
 skipMembers:=func()(int,int,error){
  if p+2>len(data){return 0,0,errBCSObserverUnavailable};at:=p;n:=int(u2(p));p+=2
  for i:=0;i<n;i++{
   if p+8>len(data){return 0,0,errBCSObserverUnavailable};attrs:=int(u2(p+6));p+=8
   for j:=0;j<attrs;j++{
    if p+6>len(data){return 0,0,errBCSObserverUnavailable};size:=uint64(binary.BigEndian.Uint32(data[p+2:]));p+=6
    if size>uint64(len(data)-p){return 0,0,errBCSObserverUnavailable};p+=int(size)
   }
  }
  return at,n,nil
 }
 if _,_,err:=skipMembers();err!=nil{return nil,err}
 methodsAt,n,err:=skipMembers();if err!=nil||n==65535{return nil,errBCSObserverUnavailable}
 methodsEnd:=p
 var extra []byte
 put2:=func(out *[]byte,v uint16){*out=append(*out,byte(v>>8),byte(v))}
 put4:=func(out *[]byte,v uint32){*out=append(*out,byte(v>>24),byte(v>>16),byte(v>>8),byte(v))}
 next:=count
 add:=func(tag byte,body []byte)uint16{index:=uint16(next);next++;extra=append(extra,tag);extra=append(extra,body...);return index}
 utf:=func(text string)uint16{var b []byte;put2(&b,uint16(len(text)));b=append(b,[]byte(text)...);return add(1,b)}
 one:=func(tag byte,a uint16)uint16{var b []byte;put2(&b,a);return add(tag,b)}
 pair:=func(tag byte,a,b uint16)uint16{var v []byte;put2(&v,a);put2(&v,b);return add(tag,v)}
 wrapperName:=utf(wrapper);wrapperNT:=pair(12,wrapperName,desc)
 oldNT:=pair(12,name,desc);oldCall:=pair(10,owner,oldNT)
 probeName:=utf("org/openomsi/bridge/events/EventProbe");probe:=one(7,probeName)
 launch:=utf("launch");void:=utf("()V");launchNT:=pair(12,launch,void);launchCall:=pair(10,probe,launchNT)
 throwableName:=utf("java/lang/Throwable");throwable:=one(7,throwableName)
 codeName:=utf("Code");stackMapName:=utf("StackMapTable")
 if next>65535{return nil,errBCSObserverUnavailable}
 code:=[]byte{0xb8,byte(launchCall>>8),byte(launchCall),0xa7,0,4,0x57,0x2a,0xb8,byte(oldCall>>8),byte(oldCall),0xb0}
 var frames []byte;put2(&frames,2);frames=append(frames,70,7);put2(&frames,throwable);frames=append(frames,0)
 var body []byte;put2(&body,1);put2(&body,1);put4(&body,uint32(len(code)));body=append(body,code...)
 put2(&body,1);put2(&body,0);put2(&body,3);put2(&body,6);put2(&body,throwable)
 put2(&body,1);put2(&body,stackMapName);put4(&body,uint32(len(frames)));body=append(body,frames...)
 var method []byte;put2(&method,0x000a);put2(&method,wrapperName);put2(&method,desc);put2(&method,1);put2(&method,codeName);put4(&method,uint32(len(body)));method=append(method,body...)
 original:=append([]byte(nil),data...)
 // Redirect the one reference added by our extension, retaining its original method.
 binary.BigEndian.PutUint16(original[call+3:],wrapperNT)
 binary.BigEndian.PutUint16(original[methodsAt:],uint16(n+1))
 binary.BigEndian.PutUint16(original[8:],uint16(next))
 out:=append([]byte(nil),original[:cpEnd]...);out=append(out,extra...)
 out=append(out,original[cpEnd:methodsEnd]...);out=append(out,method...);out=append(out,original[methodsEnd:]...)
 return out,nil
}

func bcsCaptureEntries(logger []byte)(map[string][]byte,error){
 data,err:=base64.StdEncoding.DecodeString(bcsCapturePayload);if err!=nil{return nil,err}
 reader,err:=zip.NewReader(bytes.NewReader(data),int64(len(data)));if err!=nil{return nil,err}
 result:=map[string][]byte{bcsLoggerEntry:logger}
 for _,entry:=range reader.File{
  // The embedded archive is ours, with no vendor classes or manifest overrides.
  if entry.Name!="bcs-event-fingerprints.properties"&&!(strings.HasPrefix(entry.Name,"org/openomsi/bridge/events/")&&strings.HasSuffix(entry.Name,".class")){
   return nil,fmt.Errorf("recurso de captura inesperado")
  }
  if entry.UncompressedSize64>256<<10{return nil,fmt.Errorf("recurso de captura grande demais")}
  r,err:=entry.Open();if err!=nil{return nil,err};payload,err:=io.ReadAll(r);r.Close();if err!=nil{return nil,err};result[entry.Name]=payload
 }
 return result,nil
}
