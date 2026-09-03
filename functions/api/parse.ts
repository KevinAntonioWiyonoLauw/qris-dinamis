import { parseQRIS } from "../../frontend/src/lib/qris";

export const onRequestPost = async ({ request }: { request: Request }) => {
  try {
    const body = await request.json() as { qris?: string };
    return Response.json({ data: parseQRIS(body.qris ?? "") });
  } catch {
    return Response.json({ error: "invalid QRIS" }, { status: 400 });
  }
};
