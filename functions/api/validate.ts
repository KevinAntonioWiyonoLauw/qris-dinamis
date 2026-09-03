import { validateQRIS } from "../../frontend/src/lib/qris";

export const onRequestPost = async ({ request }: { request: Request }) => {
  try {
    const body = await request.json() as { qris?: string };
    return Response.json(validateQRIS(body.qris ?? ""));
  } catch {
    return Response.json({ error: "invalid JSON body" }, { status: 400 });
  }
};
