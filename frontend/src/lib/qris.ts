// Offline QRIS core for browser. Mirrors the Go/TS reference behavior exactly.
export type TLVPair = { tag: string; name: string; length: number; value: string; children?: TLVPair[] };
export type MerchantInfo = { tag: string; globallyUniqueId: string; merchantId?: string; merchantCriteria?: string; fields: TLVPair[] };
export type QRISMethod = "static" | "dynamic";
export type QRISData = {
  version: string;
  method: QRISMethod;
  merchantAccountInfo: MerchantInfo[];
  merchantCategoryCode: string;
  currency: string;
  amount?: string;
  tipIndicator?: "prompt" | "fixed" | "percentage";
  tipFixed?: string;
  tipPercentage?: string;
  countryCode: string;
  merchantName: string;
  merchantCity: string;
  postalCode: string;
  additionalData?: TLVPair[];
  crc: string;
  raw: TLVPair[];
};
export type FeeType = "fixed" | "percentage";
export type ConvertOptions = { amount: string; fee?: { type: FeeType; value: number } };
export type StrictResult = { errors: string[]; warnings: string[] };

const TAG_NAMES: Record<string, string> = {
  "00": "Payload Format Indicator", "01": "Point of Initiation Method", "02": "Visa", "03": "Mastercard",
  "04": "Mastercard", "15": "Visa", "52": "Merchant Category Code", "53": "Transaction Currency",
  "54": "Transaction Amount", "55": "Tip or Convenience Indicator", "56": "Value of Convenience Fee (Fixed)",
  "57": "Value of Convenience Fee (%)", "58": "Country Code", "59": "Merchant Name", "60": "Merchant City",
  "61": "Postal Code", "62": "Additional Data Field", "63": "CRC",
};
for (let i = 26; i <= 51; i++) TAG_NAMES[String(i).padStart(2, "0")] = "Merchant Account Information";

export function calculateCRC16(str: string): string {
  let crc = 0xffff;
  for (let i = 0; i < str.length; i++) {
    crc ^= str.charCodeAt(i) << 8;
    for (let j = 0; j < 8; j++) {
      if (crc & 0x8000) crc = ((crc << 1) ^ 0x1021) & 0xffff;
      else crc = (crc << 1) & 0xffff;
    }
  }
  return (crc & 0xffff).toString(16).toUpperCase().padStart(4, "0");
}

function isNestedTag(tag: string): boolean {
  if (tag === "62") return true;
  const n = parseInt(tag, 10);
  return n >= 26 && n <= 51;
}

export function parseTLV(data: string): TLVPair[] {
  const elements: TLVPair[] = [];
  let pos = 0;
  while (pos < data.length) {
    if (pos + 4 > data.length) break;
    const tag = data.substring(pos, pos + 2);
    const length = parseInt(data.substring(pos + 2, pos + 4), 10);
    if (isNaN(length) || pos + 4 + length > data.length) break;
    const value = data.substring(pos + 4, pos + 4 + length);
    const name = TAG_NAMES[tag] ?? `Unknown (${tag})`;
    const element: TLVPair = { tag, name, length, value };
    if (isNestedTag(tag)) element.children = parseTLV(value);
    elements.push(element);
    pos += 4 + length;
  }
  return elements;
}

export function parseQRIS(qrisString: string): QRISData {
  const raw = parseTLV(qrisString);
  const findTag = (tag: string) => raw.find((t) => t.tag === tag);
  const method: QRISMethod = findTag("01")?.value === "12" ? "dynamic" : "static";
  let tipIndicator: QRISData["tipIndicator"];
  const tipVal = findTag("55")?.value;
  if (tipVal === "01") tipIndicator = "prompt";
  else if (tipVal === "02") tipIndicator = "fixed";
  else if (tipVal === "03") tipIndicator = "percentage";
  const merchantAccountInfo: MerchantInfo[] = raw
    .filter((t) => {
      const n = parseInt(t.tag, 10);
      return n >= 26 && n <= 51 && !!t.children;
    })
    .map((t) => {
      const children = t.children ?? [];
      const findChild = (c: string) => children.find((x) => x.tag === c);
      return {
        tag: t.tag,
        globallyUniqueId: findChild("00")?.value ?? "",
        merchantId: findChild("01")?.value ?? findChild("02")?.value,
        merchantCriteria: findChild("03")?.value,
        fields: children,
      };
    });
  return {
    version: findTag("00")?.value ?? "01",
    method,
    merchantAccountInfo,
    merchantCategoryCode: findTag("52")?.value ?? "",
    currency: findTag("53")?.value ?? "360",
    amount: findTag("54")?.value,
    tipIndicator,
    tipFixed: findTag("56")?.value,
    tipPercentage: findTag("57")?.value,
    countryCode: findTag("58")?.value ?? "ID",
    merchantName: findTag("59")?.value ?? "",
    merchantCity: findTag("60")?.value ?? "",
    postalCode: findTag("61")?.value ?? "",
    additionalData: findTag("62")?.children,
    crc: findTag("63")?.value ?? "",
    raw,
  };
}

function buildTLVString(elements: TLVPair[]): string {
  return elements
    .map((el) => {
      const value = el.children ? buildTLVString(el.children) : el.value;
      const length = value.length.toString().padStart(2, "0");
      return `${el.tag}${length}${value}`;
    })
    .join("");
}

function makeTLV(tag: string, value: string): TLVPair {
  return { tag, name: "", length: value.length, value };
}

export function formatFee(v: number): string {
  return Number.isInteger(v) ? String(v) : String(v);
}

export function convertQRIS(qrisString: string, options: ConvertOptions): string {
  const elements = parseTLV(qrisString);
  const result: TLVPair[] = [];
  let amountInserted = false;
  const managed = new Set(["54", "55", "56", "57", "63"]);
  for (const el of elements) {
    if (managed.has(el.tag)) continue;
    if (el.tag === "01") {
      result.push(makeTLV("01", "12"));
      continue;
    }
    if (el.tag === "58" && !amountInserted) {
      result.push(makeTLV("54", options.amount || "0"));
      if (options.fee) {
        if (options.fee.type === "fixed") {
          result.push(makeTLV("55", "02"));
          result.push(makeTLV("56", formatFee(options.fee.value)));
        } else {
          result.push(makeTLV("55", "03"));
          result.push(makeTLV("57", formatFee(options.fee.value)));
        }
      }
      amountInserted = true;
    }
    result.push(el);
  }
  const crcInput = buildTLVString(result) + "6304";
  return crcInput + calculateCRC16(crcInput);
}

export function validateQRIS(qrisString: string): { valid: boolean; errors: string[] } {
  const errors: string[] = [];
  const str = qrisString.trim();
  if (!str) return { valid: false, errors: ["QRIS string is empty"] };
  if (!str.startsWith("000201")) errors.push('QRIS must start with Payload Format Indicator "000201"');
  if (str.length < 20) return { valid: false, errors: [...errors, "QRIS string is too short"] };
  const declared = str.slice(-4).toUpperCase();
  const calculated = calculateCRC16(str.slice(0, -4));
  if (declared !== calculated) errors.push(`CRC mismatch: expected ${calculated}, got ${declared}`);
  const elements = parseTLV(str);
  if (elements.length === 0) return { valid: false, errors: [...errors, "Failed to parse any TLV elements"] };
  const tags = new Set(elements.map((e) => e.tag));
  const required = [
    ["00", "Payload Format Indicator"], ["01", "Point of Initiation Method"], ["52", "Merchant Category Code"],
    ["53", "Transaction Currency"], ["58", "Country Code"], ["59", "Merchant Name"], ["60", "Merchant City"], ["63", "CRC"],
  ];
  for (const [tag, name] of required) if (!tags.has(tag)) errors.push(`Missing required tag ${tag} (${name})`);
  const methodEl = elements.find((e) => e.tag === "01");
  if (methodEl && methodEl.value !== "11" && methodEl.value !== "12") {
    errors.push(`Invalid Point of Initiation Method: "${methodEl.value}" (must be "11" or "12")`);
  }
  const hasMerchant = elements.some((e) => {
    const n = parseInt(e.tag, 10);
    return n >= 26 && n <= 51;
  });
  if (!hasMerchant) errors.push("No Merchant Account Information found (tags 26-51)");
  return { valid: errors.length === 0, errors };
}

export function strictValidate(qrisString: string): StrictResult {
  const { errors } = validateQRIS(qrisString);
  const warnings: string[] = [];
  const str = qrisString.trim();
  const data = parseQRIS(str);
  if (data.currency && data.currency !== "360") warnings.push(`Currency bukan IDR (${data.currency}). QRIS umumnya harus 360.`);
  if (data.countryCode && data.countryCode !== "ID") warnings.push(`Country code bukan ID (${data.countryCode}).`);
  if (!data.merchantName.trim()) warnings.push("Nama merchant kosong.");
  if (data.amount && !/^\d+$/.test(data.amount)) warnings.push("Jumlah mengandung karakter non-digit.");
  const merchantTags = data.raw.filter((t) => {
    const n = parseInt(t.tag, 10);
    return n >= 26 && n <= 51;
  });
  const seen = new Set<string>();
  for (const t of merchantTags) {
    if (seen.has(t.tag)) warnings.push(`Tag merchant ganda ditemukan: ${t.tag}.`);
    seen.add(t.tag);
  }
  if (data.tipIndicator === "fixed" && data.tipFixed && !/^\d+$/.test(data.tipFixed)) warnings.push("Biaya tetap bukan angka bulat valid.");
  return { errors, warnings };
}