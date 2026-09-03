import qrcode from "qrcode-generator";

type Query = { data?: string | null; size?: string | null };

export const onRequestGet = async ({ request }: { request: Request }) => {
  const url = new URL(request.url);
  const { data, size } = Object.fromEntries(url.searchParams) as Query;
  if (!data) return Response.json({ error: "missing data query param" }, { status: 400 });
  let cellCount = 280;
  if (size) {
    const n = Number.parseInt(size, 10);
    if (Number.isInteger(n) && n > 0 && n <= 2048) cellCount = n;
  }
  try {
    const qr = qrcode(0, "M");
    qr.addData(data);
    qr.make();
    const svg = qr.createSvgTag({ cellSize: 2, margin: 2 });
    // scale SVG to requested size, replacing the intrinsic size attributes
    const sized = svg.replace(/\swidth="[^"]*"\sheight="[^"]*"/, ` width="${cellCount}" height="${cellCount}"`);
    return new Response(sized, {
      headers: { "Content-Type": "image/svg+xml", "Cache-Control": "no-store" },
    });
  } catch {
    return Response.json({ error: "failed to generate QR" }, { status: 400 });
  }
};