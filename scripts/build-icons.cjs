// Requires sharp; generates app formats from the ImageGen bunny master.
const fs = require('node:fs');
const path = require('node:path');
const sharp = require('sharp');
const root = path.resolve(__dirname, '..');
async function main() {
  const source = path.join(root, 'internal/brand/source.png');
  if (!process.argv.includes("--mac-only")) {
    const png = await sharp(source).resize(1024,1024).png().toBuffer();
    fs.writeFileSync(path.join(root,'internal/brand/icon.png'),png);
    fs.writeFileSync(path.join(root,'internal/brand/icon-ui.png'),await sharp(source).resize(128,128).png().toBuffer());
    const sizes=[16,24,32,48,64,128,256];
    const images=await Promise.all(sizes.map(size=>sharp(source).resize(size,size).png().toBuffer()));
    const header=Buffer.alloc(6+16*sizes.length); header.writeUInt16LE(1,2);header.writeUInt16LE(sizes.length,4);
    let offset=header.length;
    images.forEach((image,i)=>{let at=6+16*i;header[at]=sizes[i]===256?0:sizes[i];header[at+1]=header[at];header.writeUInt16LE(1,at+4);header.writeUInt16LE(32,at+6);header.writeUInt32LE(image.length,at+8);header.writeUInt32LE(offset,at+12);offset+=image.length;});
    fs.writeFileSync(path.join(root,'cmd/desktop/app.ico'),Buffer.concat([header,...images]));
  }
  // Use the standard macOS icon silhouette. The alpha margins and continuous
  // corners let macOS render the artwork without a separate legacy plate.
  const corner = [
    [350,100], [308,102], [281,105], [255,110], [224,120], [204,130],
    [188,140], [175,150], [163,163], [150,175], [140,188], [130,204],
    [120,224], [110,255], [105,281], [102,308], [100,350],
  ];
  const outline = [
    ...corner,
    ...corner.map(([x, y]) => [y, 1024 - x]),
    ...corner.map(([x, y]) => [1024 - x, 1024 - y]),
    ...corner.map(([x, y]) => [1024 - y, x]),
  ].map(point => point.join(',')).join(' ');
  const mask = Buffer.from(`<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024">
    <polygon fill="white" points="${outline}" transform="translate(512 512) scale(1.02) translate(-512 -512)"/>
  </svg>`);
  const artwork = await sharp(source).flatten({background: '#9fb5a0'}).resize(824, 824)
    .extend({top: 100, bottom: 100, left: 100, right: 100, background: '#9fb5a0'})
    .ensureAlpha().png().toBuffer();
  const macIcon = await sharp(artwork).composite([{input: mask, blend: 'dest-in'}]).png().toBuffer();
  const chunks=[];
  const macSizes = [
    ['icp4',16], ['icp5',32], ['icp6',64], ['ic07',128], ['ic08',256], ['ic09',512], ['ic10',1024],
    ['ic11',32], ['ic12',64], ['ic13',256], ['ic14',512],
  ];
  for (const [type, size] of macSizes) {
    const image = await sharp(macIcon).resize(size, size).png().toBuffer();
    const chunk = Buffer.alloc(8);
    chunk.write(type);
    chunk.writeUInt32BE(image.length + 8, 4);
    chunks.push(chunk, image);
  }
  const icns=Buffer.alloc(8);icns.write('icns');icns.writeUInt32BE(chunks.reduce((n,c)=>n+c.length,8),4);
  fs.writeFileSync(path.join(root,'packaging/darwin/MyGittu.icns'),Buffer.concat([icns,...chunks]));
}
main().catch(e=>{console.error(e);process.exitCode=1;});
