// Requires sharp; generates app formats from the ImageGen bunny master.
const fs = require('node:fs');
const path = require('node:path');
const sharp = require('sharp');
const root = path.resolve(__dirname, '..');
async function main() {
  const source = path.join(root, 'internal/brand/source.png');
  const png = await sharp(source).resize(1024,1024).png().toBuffer();
  fs.writeFileSync(path.join(root,'internal/brand/icon.png'),png);
  fs.writeFileSync(path.join(root,'internal/brand/icon-ui.png'),await sharp(source).resize(128,128).png().toBuffer());
  const sizes=[16,24,32,48,64,128,256];
  const images=await Promise.all(sizes.map(size=>sharp(source).resize(size,size).png().toBuffer()));
  const header=Buffer.alloc(6+16*sizes.length); header.writeUInt16LE(1,2);header.writeUInt16LE(sizes.length,4);
  let offset=header.length;
  images.forEach((image,i)=>{let at=6+16*i;header[at]=sizes[i]===256?0:sizes[i];header[at+1]=header[at];header.writeUInt16LE(1,at+4);header.writeUInt16LE(32,at+6);header.writeUInt32LE(image.length,at+8);header.writeUInt32LE(offset,at+12);offset+=image.length;});
  fs.writeFileSync(path.join(root,'cmd/desktop/app.ico'),Buffer.concat([header,...images]));
  const chunks=[];
  for(const [type,size] of [['icp4',16],['icp5',32],['icp6',64],['ic07',128],['ic08',256],['ic09',512],['ic10',1024]]) {
    const image=await sharp(source).resize(size,size).png().toBuffer();const chunk=Buffer.alloc(8);chunk.write(type);chunk.writeUInt32BE(image.length+8,4);chunks.push(chunk,image);
  }
  const icns=Buffer.alloc(8);icns.write('icns');icns.writeUInt32BE(chunks.reduce((n,c)=>n+c.length,8),4);
  fs.writeFileSync(path.join(root,'packaging/darwin/MyGittu.icns'),Buffer.concat([icns,...chunks]));
}
main().catch(e=>{console.error(e);process.exitCode=1;});
