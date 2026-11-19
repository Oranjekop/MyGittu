// Requires sharp, like build-icons.cjs. Rebuilds the Windows command ICOs.
const fs = require('node:fs/promises');
const path = require('node:path');
const sharp = require('sharp');

const output = path.join(__dirname, '..', 'extensions', 'windows', 'menu-icons');
const sizes = [16, 20, 24, 28, 32, 36, 40, 48, 64, 96, 128, 192, 256];
const icons = {
  pull: ['#d9f5e7', '#08704b', '<path d="M12 4.5v10m-4-4 4 4 4-4M5.5 15v3.8h13V15"/>'],
  changes: ['#e3edff', '#2452a0', '<path d="M6.5 5h7l4 4v10h-11zM13.5 5v4h4m-8 3.5h5m-5 3h4"/>'],
  commit: ['#eee5ff', '#6437a0', '<path d="M3.8 12h3m10.4 0h3"/><circle cx="12" cy="12" r="5.2"/><path d="m9.6 12 1.6 1.7 3.2-3.4"/>'],
  push: ['#e6eaff', '#373d9e', '<path d="M12 14.5v-10m-4 4 4-4 4 4M5.5 15v3.8h13V15"/>'],
  history: ['#e0f4ff', '#12657e', '<path d="M5.2 9.5a7.1 7.1 0 1 1 .9 7.3M5.1 5.7v4.4h4.4M12 8v4l2.5 1.6"/>'],
  diff: ['#ffeedc', '#98431a', '<rect x="4.8" y="5.8" width="14.4" height="12.4" rx="1.5"/><path d="M12 6v12m-5-6h2.7m4.6 0h2.8m-1.4-1.4v2.8"/>'],
  clone: ['#daf5f3', '#006d68', '<path d="M12 5H7a2 2 0 0 0-2 2v5"/><rect x="8.5" y="8.5" width="10.5" height="10.5" rx="2"/><path d="M11.5 13.75H16m-2.25-2.25V16"/>'],
};

function iconSVG(name) {
  const [background, stroke, drawing] = icons[name];
  return Buffer.from(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24">
  <rect x=".8" y=".8" width="22.4" height="22.4" rx="5.5" fill="${background}"/>
  <g fill="none" stroke="${stroke}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${drawing}</g>
</svg>`);
}

function makeICO(images) {
  const directory = Buffer.alloc(6 + 16 * sizes.length);
  directory.writeUInt16LE(1, 2);
  directory.writeUInt16LE(sizes.length, 4);
  let offset = directory.length;
  for (const [i, image] of images.entries()) {
    const entry = 6 + 16 * i;
    directory[entry] = directory[entry + 1] = sizes[i] === 256 ? 0 : sizes[i];
    directory.writeUInt16LE(1, entry + 4);
    directory.writeUInt16LE(32, entry + 6);
    directory.writeUInt32LE(image.length, entry + 8);
    directory.writeUInt32LE(offset, entry + 12);
    offset += image.length;
  }
  return Buffer.concat([directory, ...images]);
}

async function raster(name, size) {
  return sharp(iconSVG(name), { density: 768 }).resize(size, size).png().toBuffer();
}

async function main() {
  await fs.mkdir(output, { recursive: true });
  await Promise.all(Object.keys(icons).map(async name => {
    const images = await Promise.all(sizes.map(size => raster(name, size)));
    await fs.writeFile(path.join(output, name + '.ico'), makeICO(images));
  }));
  if (process.argv[2]) {
    const layers = await Promise.all(Object.keys(icons).map(async (name, index) => ({
      input: await raster(name, 48), left: 12 + index * 62, top: 12,
    })));
    await sharp({ create: { width: 448, height: 72, channels: 4, background: '#f4f4f4' } })
      .composite(layers).png().toFile(process.argv[2]);
  }
}

main().catch(error => { console.error(error); process.exitCode = 1; });
