import { convertQRIS, parseQRIS, validateQRIS } from "../../frontend/src/lib/qris";

type Body = { qris?: string; amount?: string; fee?: { type: "fixed" | "percentage"; value: number } };

export const onRequestPost = async ({ request }: { request: Request }) => {
  try {
    const body = await request.json() as Body;
    const qris = body.qris?.trim() ?? "";
    const amount = body.amount?.trim() ?? "";
    const validation = validateQRIS(qris);
    if (!validation.valid) return Response.json(validation, { status: 400 });
    if (parseQRIS(qris).method === "dynamic") return Response.json({ error: "QRIS is already dynamic" }, { status: 400 });
    if (!/^\d+$/.test(amount) || Number(amount) <= 0) return Response.json({ error: "amount must be a positive integer" }, { status: 400 });
    const result = convertQRIS(qris, { amount, fee: body.fee });
    return Response.json({ result, parsed: parseQRIS(result) });
  } catch {
    return Response.json({ error: "invalid QRIS request" }, { status: 400 });
  }
};
