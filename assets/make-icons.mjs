import { deflateSync } from "node:zlib";
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ink = [0x0e, 0x2f, 0x36, 0xff];
const glyph = [0xe8, 0xf1, 0xef, 0xff];
const found = [0xf2, 0xb1, 0x05, 0xff];

const supersample = 4;

function canvas(width, height) {
	const data = new Uint8Array(width * height * 4);
	return { width, height, data };
}

function put(image, x, y, colour) {
	if (x < 0 || y < 0 || x >= image.width || y >= image.height) {
		return;
	}
	const at = (y * image.width + x) * 4;
	image.data.set(colour, at);
}

function fillRounded(image, x0, y0, w, h, radius, colour) {
	for (let y = y0; y < y0 + h; y++) {
		for (let x = x0; x < x0 + w; x++) {
			const dx = Math.max(x0 + radius - x, x - (x0 + w - 1 - radius), 0);
			const dy = Math.max(y0 + radius - y, y - (y0 + h - 1 - radius), 0);
			if (dx * dx + dy * dy <= radius * radius) {
				put(image, x, y, colour);
			}
		}
	}
}

function fillCircle(image, cx, cy, radius, colour) {
	for (let y = Math.floor(cy - radius); y <= Math.ceil(cy + radius); y++) {
		for (let x = Math.floor(cx - radius); x <= Math.ceil(cx + radius); x++) {
			const dx = x - cx;
			const dy = y - cy;
			if (dx * dx + dy * dy <= radius * radius) {
				put(image, x, y, colour);
			}
		}
	}
}

function dashedSquare(image, x0, y0, size, stroke, dash, gap, colour) {
	const corner = dash;
	const on = (t) => t < corner || t >= size - corner || t % (dash + gap) < dash;
	for (let t = 0; t < size; t++) {
		if (!on(t)) {
			continue;
		}
		for (let s = 0; s < stroke; s++) {
			put(image, x0 + t, y0 + s, colour);
			put(image, x0 + t, y0 + size - 1 - s, colour);
			put(image, x0 + s, y0 + t, colour);
			put(image, x0 + size - 1 - s, y0 + t, colour);
		}
	}
}

function downsample(image, factor) {
	const width = image.width / factor;
	const height = image.height / factor;
	const out = canvas(width, height);
	for (let y = 0; y < height; y++) {
		for (let x = 0; x < width; x++) {
			const total = [0, 0, 0, 0];
			for (let sy = 0; sy < factor; sy++) {
				for (let sx = 0; sx < factor; sx++) {
					const at = ((y * factor + sy) * image.width + x * factor + sx) * 4;
					for (let c = 0; c < 4; c++) {
						total[c] += image.data[at + c];
					}
				}
			}
			const n = factor * factor;
			out.data.set(total.map((v) => Math.round(v / n)), (y * width + x) * 4);
		}
	}
	return out;
}

function mark(image, cx, cy, size) {
	const box = Math.round(size);
	const stroke = Math.max(1, Math.round(size * 0.088));
	const dash = Math.round(size * 0.19);
	dashedSquare(image, Math.round(cx - box / 2), Math.round(cy - box / 2), box, stroke, dash, dash, glyph);
	fillCircle(image, cx, cy, size * 0.185, found);
}

function square(side, radius) {
	const big = canvas(side * supersample, side * supersample);
	const s = side * supersample;
	fillRounded(big, 0, 0, s, s, radius * supersample, ink);
	mark(big, s / 2, s / 2, s * 0.68);
	return downsample(big, supersample);
}

function banner(width, height) {
	const big = canvas(width * supersample, height * supersample);
	fillRounded(big, 0, 0, width * supersample, height * supersample, 0, ink);
	mark(big, (width * supersample) / 2, (height * supersample) / 2, height * supersample * 0.56);
	return downsample(big, supersample);
}

const crcTable = Array.from({ length: 256 }, (_, n) => {
	let c = n;
	for (let k = 0; k < 8; k++) {
		c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
	}
	return c >>> 0;
});

function crc32(buf) {
	let c = 0xffffffff;
	for (const byte of buf) {
		c = crcTable[(c ^ byte) & 0xff] ^ (c >>> 8);
	}
	return (c ^ 0xffffffff) >>> 0;
}

function chunk(type, body) {
	const length = Buffer.alloc(4);
	length.writeUInt32BE(body.length);
	const typed = Buffer.concat([Buffer.from(type, "ascii"), body]);
	const crc = Buffer.alloc(4);
	crc.writeUInt32BE(crc32(typed));
	return Buffer.concat([length, typed, crc]);
}

// Colour type 6 is RGBA, bit depth 8, no interlacing. Each scanline carries a
// leading filter byte, which is 0 for none.
function png(image) {
	const header = Buffer.alloc(13);
	header.writeUInt32BE(image.width, 0);
	header.writeUInt32BE(image.height, 4);
	header[8] = 8;
	header[9] = 6;

	const stride = image.width * 4;
	const raw = Buffer.alloc((stride + 1) * image.height);
	for (let y = 0; y < image.height; y++) {
		raw[y * (stride + 1)] = 0;
		Buffer.from(image.data.buffer, y * stride, stride).copy(raw, y * (stride + 1) + 1);
	}

	return Buffer.concat([
		Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
		chunk("IHDR", header),
		chunk("IDAT", deflateSync(raw, { level: 9 })),
		chunk("IEND", Buffer.alloc(0)),
	]);
}

function toolbarSvg(fill) {
	const bracket = 6;
	const stroke = 2;
	const edge = 22;
	const near = 1;
	const far = edge - near - stroke;
	const arms = [
		`M${near} ${near}h${bracket}v${stroke}h-${bracket - stroke}v${bracket - stroke}h-${stroke}z`,
		`M${far + stroke} ${near}h-${bracket}v${stroke}h${bracket - stroke}v${bracket - stroke}h${stroke}z`,
		`M${near} ${far + stroke}h${bracket}v-${stroke}h-${bracket - stroke}v-${bracket - stroke}h-${stroke}z`,
		`M${far + stroke} ${far + stroke}h-${bracket}v-${stroke}h${bracket - stroke}v-${bracket - stroke}h${stroke}z`,
	];
	return [
		'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24">',
		`  <g fill="${fill}">`,
		...arms.map((d) => `    <path d="${d}"/>`),
		'    <circle cx="12" cy="12" r="4"/>',
		"  </g>",
		"</svg>",
		"",
	].join("\n");
}

const here = dirname(fileURLToPath(import.meta.url));
const outputs = [
	[join(here, "..", "editors", "vscode", "media", "icon.png"), square(256, 48)],
	[join(here, "icon.png"), square(512, 96)],
	[join(here, "social-preview.png"), banner(1280, 640)],
];

for (const [path, image] of outputs) {
	mkdirSync(dirname(path), { recursive: true });
	const bytes = png(image);
	writeFileSync(path, bytes);
	console.log(`${path} ${image.width}x${image.height} ${bytes.length} bytes`);
}

const media = join(here, "..", "editors", "vscode", "media");
for (const [name, fill] of [
	["toolbar-light.svg", "#424242"],
	["toolbar-dark.svg", "#C5C5C5"],
]) {
	const path = join(media, name);
	writeFileSync(path, toolbarSvg(fill));
	console.log(`${path} svg`);
}
