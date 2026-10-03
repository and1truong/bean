// Minimal zip reader/writer for source import/export. The writer emits
// STORE (uncompressed) entries; the reader accepts STORE and DEFLATE entries
// using DecompressionStream. Multi-disk archives, encryption, and Zip64 are
// rejected — playground sources are a handful of small text files.

const encoder=new TextEncoder()
const decoder=new TextDecoder()

const crcTable=(()=>{
  const table=new Uint32Array(256)
  for(let n=0;n<256;n++){let c=n;for(let k=0;k<8;k++)c=c&1?0xedb88320^(c>>>1):c>>>1;table[n]=c>>>0}
  return table
})()

function crc32(data:Uint8Array):number{
  let crc=0xffffffff
  for(let i=0;i<data.length;i++)crc=crcTable[(crc^data[i])&0xff]^(crc>>>8)
  return (crc^0xffffffff)>>>0
}

function dosDate(date=new Date()){return {time:(date.getHours()<<11)|(date.getMinutes()<<5)|(date.getSeconds()>>1),date:((date.getFullYear()-1980)<<9)|((date.getMonth()+1)<<5)|date.getDate()}}

export async function writeZip(entries:Record<string,string>):Promise<Blob>{
  const names=Object.keys(entries).sort()
  const parts:BlobPart[]=[]
  const central:BlobPart[]=[]
  let centralSize=0
  let offset=0
  const {time,date}=dosDate()
  for(const name of names){
    const nameBytes=encoder.encode(name)
    const data=encoder.encode(entries[name])
    const crc=crc32(data)
    const local=new DataView(new ArrayBuffer(30))
    local.setUint32(0,0x04034b50,true);local.setUint16(4,20,true);local.setUint16(6,0,true);local.setUint16(8,0,true)
    local.setUint16(10,time,true);local.setUint16(12,date,true)
    local.setUint32(14,crc,true);local.setUint32(18,data.length,true);local.setUint32(22,data.length,true)
    local.setUint16(26,nameBytes.length,true);local.setUint16(28,0,true)
    parts.push(new Uint8Array(local.buffer),nameBytes,data)
    const head=new DataView(new ArrayBuffer(46))
    head.setUint32(0,0x02014b50,true);head.setUint16(4,20,true);head.setUint16(6,20,true);head.setUint16(8,0,true);head.setUint16(10,0,true)
    head.setUint16(12,time,true);head.setUint16(14,date,true)
    head.setUint32(16,crc,true);head.setUint32(20,data.length,true);head.setUint32(24,data.length,true)
    head.setUint16(28,nameBytes.length,true);head.setUint32(42,offset,true)
    central.push(new Uint8Array(head.buffer),nameBytes)
    centralSize+=46+nameBytes.length
    offset+=30+nameBytes.length+data.length
  }
  const end=new DataView(new ArrayBuffer(22))
  end.setUint32(0,0x06054b50,true)
  end.setUint16(8,names.length,true);end.setUint16(10,names.length,true)
  end.setUint32(12,centralSize,true);end.setUint32(16,offset,true)
  return new Blob([...parts,...central,new Uint8Array(end.buffer)],{type:'application/zip'})
}

export async function readZip(file:Blob):Promise<Record<string,string>>{
  const buffer=new Uint8Array(await file.arrayBuffer())
  const view=new DataView(buffer.buffer,buffer.byteOffset,buffer.byteLength)
  // The end-of-central-directory record is within the last 64 KiB + 22 bytes.
  let eocd=-1
  for(let i=buffer.length-22;i>=Math.max(0,buffer.length-22-65536);i--){
    if(view.getUint32(i,true)===0x06054b50){eocd=i;break}
  }
  if(eocd<0)throw new Error('not a zip archive')
  const count=view.getUint16(eocd+10,true)
  let cursor=view.getUint32(eocd+16,true)
  const files:Record<string,string>={}
  for(let i=0;i<count;i++){
    if(view.getUint32(cursor,true)!==0x02014b50)throw new Error('corrupt zip central directory')
    const flags=view.getUint16(cursor+8,true)
    const method=view.getUint16(cursor+10,true)
    const compressed=view.getUint32(cursor+20,true)
    const nameLength=view.getUint16(cursor+28,true)
    const extraLength=view.getUint16(cursor+30,true)
    const commentLength=view.getUint16(cursor+32,true)
    const localOffset=view.getUint32(cursor+42,true)
    const name=decoder.decode(buffer.subarray(cursor+46,cursor+46+nameLength))
    if(flags&1)throw new Error('encrypted zip entries are not supported')
    if(method!==0&&method!==8)throw new Error(`unsupported zip compression method ${method} for ${name}`)
    if(view.getUint32(localOffset,true)!==0x04034b50)throw new Error('corrupt zip local header')
    const localNameLength=view.getUint16(localOffset+26,true)
    const localExtraLength=view.getUint16(localOffset+28,true)
    const dataStart=localOffset+30+localNameLength+localExtraLength
    const data=buffer.subarray(dataStart,dataStart+compressed)
    let raw:Uint8Array
    if(method===0)raw=data
    else{
      const stream=new Blob([data]).stream().pipeThrough(new DecompressionStream('deflate-raw'))
      raw=new Uint8Array(await new Response(stream).arrayBuffer())
    }
    if(!name.endsWith('/'))files[name]=decoder.decode(raw)
    cursor+=46+nameLength+extraLength+commentLength
  }
  return files
}
