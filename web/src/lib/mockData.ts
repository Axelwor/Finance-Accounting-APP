/**
 * Structured mock data for Sales, Purchases, Inventory, and Fixed Assets
 * modules. These shapes mirror what the eventual backend responses will
 * look like, but values are generated deterministically from a stable
 * seed so the UI looks the same across reloads.
 */

import { hashString, pickFrom, randInt, seedRandom } from "./random";

/* ------------------------------------------------------------------ */
/* Shared vocabulary                                                  */
/* ------------------------------------------------------------------ */

const CUSTOMERS = [
  "PT Surya Abadi",
  "CV Mitra Niaga",
  "UD Sumber Rezeki",
  "PT Lintas Samudra",
  "Toko Makmur Sentosa",
  "PT Andalan Prima",
  "CV Bintang Timur",
  "UD Jaya Bersama",
  "PT Cahaya Nusantara",
  "Toko Sederhana",
  "PT Karya Mandiri",
  "CV Gemilang Persada",
  "UD Tunas Baru",
  "PT Pilar Utama",
  "Toko Bersama",
];

const SUPPLIERS = [
  "PT Sumber Makmur",
  "CV Hasil Bumi",
  "UD Tani Jaya",
  "PT Indo Bahan",
  "CV Mitra Tani",
  "UD Pertanian Sehat",
  "PT Agro Lestari",
  "CV Niaga Bersama",
  "UD Hasil Panen",
  "PT Distribusi Utama",
  "CV Logistik Nusantara",
  "UD Karya Tani",
  "PT Suplai Mandiri",
  "CV Pasokan Hijau",
  "UD Mitra Sejati",
];

const PAYMENT_METHODS = ["Cash", "Bank BCA", "Bank Mandiri", "Bank BNI", "Bank BRI", "QRIS"];

const ITEM_CATEGORIES = ["Raw Material", "Finished Goods", "Spare Part", "Packaging", "Consumable"];

const MOVEMENT_TYPES = ["RECEIPT", "ISSUE", "TRANSFER", "ADJUSTMENT"];

const ASSET_CATEGORIES = ["Land", "Building", "Vehicle", "Office Equipment", "Machinery", "IT Equipment"];

/** Status mix — most are POSTED, a few VOID/DRAFT for visual variety. */
const STATUSES = ["POSTED", "POSTED", "POSTED", "POSTED", "POSTED", "POSTED", "POSTED", "POSTED", "VOID", "DRAFT"];

/* ------------------------------------------------------------------ */
/* Date / number helpers                                              */
/* ------------------------------------------------------------------ */

/** Today as a Date in local time. */
function today(): Date {
  return new Date();
}

/** Add `days` to a date, returning a new Date. */
function addDays(d: Date, days: number): Date {
  const out = new Date(d);
  out.setDate(out.getDate() + days);
  return out;
}

/** Format Date as yyyy-mm-dd. */
function isoDate(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
}

/** Format an IDR amount with thousands separators, no currency symbol. */
function fmt(n: number): string {
  return new Intl.NumberFormat("en-US").format(n);
}

/** A padded 4-digit number for invoice/payment IDs. */
function pad4(n: number): string {
  return String(n).padStart(4, "0");
}

/* ------------------------------------------------------------------ */
/* Sales                                                              */
/* ------------------------------------------------------------------ */

export interface SalesInvoice {
  id: string;
  number: string;
  date: string;
  customer: string;
  dueDate: string;
  amount: number;
  status: "POSTED" | "VOID" | "DRAFT";
}

export interface SalesReceipt {
  id: string;
  number: string;
  date: string;
  customer: string;
  payer: string;
  amount: number;
  status: "POSTED" | "VOID" | "DRAFT";
}

/** Deterministic sales-invoice list. 15 rows. */
export function makeSalesInvoices(seed = "sales-invoice"): SalesInvoice[] {
  const rand = seedRandom(hashString(seed));
  const base = today();
  return Array.from({ length: 15 }, (_, i) => {
    const offsetDays = randInt(rand, 1, 90) + i * 2;
    const dueOffset = randInt(rand, 14, 60);
    const issued = addDays(base, -offsetDays);
    const due = addDays(issued, dueOffset);
    const amount = randInt(rand, 250_000, 24_500_000);
    const status = pickFrom(rand, STATUSES) as SalesInvoice["status"];
    const customer = pickFrom(rand, CUSTOMERS);
    return {
      id: `sinv-${pad4(1000 + i)}`,
      number: `INV/${pad4(1000 + i)}`,
      date: isoDate(issued),
      customer,
      dueDate: isoDate(due),
      amount,
      status,
    };
  });
}

/** Deterministic sales-receipt list. 15 rows. */
export function makeSalesReceipts(seed = "sales-receipt"): SalesReceipt[] {
  const rand = seedRandom(hashString(seed));
  const base = today();
  return Array.from({ length: 15 }, (_, i) => {
    const offsetDays = randInt(rand, 1, 80) + i * 2;
    const amount = randInt(rand, 200_000, 18_000_000);
    const status = pickFrom(rand, STATUSES) as SalesReceipt["status"];
    const customer = pickFrom(rand, CUSTOMERS);
    const payer = pickFrom(rand, ["Bank BCA", "Bank Mandiri", "Bank BNI", "Cash", "QRIS"]);
    return {
      id: `srec-${pad4(2000 + i)}`,
      number: `RCP/${pad4(2000 + i)}`,
      date: isoDate(addDays(base, -offsetDays)),
      customer,
      payer,
      amount,
      status,
    };
  });
}

/* ------------------------------------------------------------------ */
/* Purchases                                                          */
/* ------------------------------------------------------------------ */

export interface PurchaseInvoice {
  id: string;
  number: string;
  date: string;
  supplier: string;
  dueDate: string;
  amount: number;
  status: "POSTED" | "VOID" | "DRAFT";
}

export interface PurchasePayment {
  id: string;
  number: string;
  date: string;
  supplier: string;
  payMethod: string;
  amount: number;
  status: "POSTED" | "VOID" | "DRAFT";
}

export function makePurchaseInvoices(seed = "purchase-invoice"): PurchaseInvoice[] {
  const rand = seedRandom(hashString(seed));
  const base = today();
  return Array.from({ length: 15 }, (_, i) => {
    const offsetDays = randInt(rand, 1, 90) + i * 2;
    const dueOffset = randInt(rand, 14, 60);
    const issued = addDays(base, -offsetDays);
    const due = addDays(issued, dueOffset);
    const amount = randInt(rand, 300_000, 22_000_000);
    const status = pickFrom(rand, STATUSES) as PurchaseInvoice["status"];
    const supplier = pickFrom(rand, SUPPLIERS);
    return {
      id: `pinv-${pad4(3000 + i)}`,
      number: `BIL/${pad4(3000 + i)}`,
      date: isoDate(issued),
      supplier,
      dueDate: isoDate(due),
      amount,
      status,
    };
  });
}

export function makePurchasePayments(seed = "purchase-payment"): PurchasePayment[] {
  const rand = seedRandom(hashString(seed));
  const base = today();
  return Array.from({ length: 15 }, (_, i) => {
    const offsetDays = randInt(rand, 1, 80) + i * 2;
    const amount = randInt(rand, 250_000, 17_500_000);
    const status = pickFrom(rand, STATUSES) as PurchasePayment["status"];
    const supplier = pickFrom(rand, SUPPLIERS);
    const payMethod = pickFrom(rand, PAYMENT_METHODS);
    return {
      id: `ppay-${pad4(4000 + i)}`,
      number: `PAY/${pad4(4000 + i)}`,
      date: isoDate(addDays(base, -offsetDays)),
      supplier,
      payMethod,
      amount,
      status,
    };
  });
}

/* ------------------------------------------------------------------ */
/* Inventory                                                          */
/* ------------------------------------------------------------------ */

export interface InventoryItem {
  id: string;
  code: string;
  name: string;
  category: string;
  onHand: number;
  unit: string;
  avgCost: number;
  status: "ACTIVE" | "INACTIVE";
}

export interface StockMovement {
  id: string;
  number: string;
  date: string;
  item: string;
  type: string;
  qty: number;
  unitCost: number;
  total: number;
  status: "POSTED" | "VOID" | "DRAFT";
}

const ITEM_NAMES = [
  "Beras Premium 5kg",
  "Minyak Goreng 1L",
  "Gula Pasir 1kg",
  "Tepung Terigu 1kg",
  "Kopi Sachet",
  "Teh Celup",
  "Sabun Mandi",
  "Deterjen Bubuk",
  "Bumbu Dapur",
  "Mie Instan",
  "Air Mineral 600ml",
  "Kecap Manis",
  "Saus Sambal",
  "Telur Ayam",
  "Margarin 250g",
];

const UNITS = ["pcs", "kg", "liter", "box", "pack", "sack"];

export function makeInventoryItems(seed = "inventory-items"): InventoryItem[] {
  const rand = seedRandom(hashString(seed));
  return ITEM_NAMES.map((name, i) => {
    const onHand = randInt(rand, 0, 1_500);
    const avgCost = randInt(rand, 500, 95_000);
    return {
      id: `item-${pad4(5000 + i)}`,
      code: `ITM-${pad4(5000 + i)}`,
      name,
      category: pickFrom(rand, ITEM_CATEGORIES),
      onHand,
      unit: pickFrom(rand, UNITS),
      avgCost,
      status: onHand > 0 ? "ACTIVE" : pickFrom(rand, ["ACTIVE", "INACTIVE"]),
    };
  });
}

export function makeStockMovements(seed = "stock-movements"): StockMovement[] {
  const rand = seedRandom(hashString(seed));
  const base = today();
  return Array.from({ length: 15 }, (_, i) => {
    const offsetDays = randInt(rand, 1, 60) + i;
    const qty = randInt(rand, 1, 250);
    const unitCost = randInt(rand, 500, 95_000);
    const status = pickFrom(rand, STATUSES) as StockMovement["status"];
    return {
      id: `stk-${pad4(6000 + i)}`,
      number: `STK/${pad4(6000 + i)}`,
      date: isoDate(addDays(base, -offsetDays)),
      item: pickFrom(rand, ITEM_NAMES),
      type: pickFrom(rand, MOVEMENT_TYPES),
      qty,
      unitCost,
      total: qty * unitCost,
      status,
    };
  });
}

/* ------------------------------------------------------------------ */
/* Fixed assets                                                       */
/* ------------------------------------------------------------------ */

export interface AssetRegisterRow {
  id: string;
  code: string;
  name: string;
  category: string;
  acquiredDate: string;
  cost: number;
  accumDep: number;
  nbv: number;
  status: "ACTIVE" | "DISPOSED" | "FULLY_DEPRECIATED";
}

const ASSET_NAMES = [
  "Gudang Pusat",
  "Kantor Cabang",
  "Toyota Avanza 2023",
  "Daihatsu Gran Max",
  "Laptop Dell Latitude",
  "Printer Canon",
  "Forklift Toyota",
  "AC Split 1PK",
  "Server Rack",
  "Meja Kerja Set",
  "Proyektor Epson",
  "Hand Pallet",
  "Genset 10kVA",
  "CCTV System",
  "Kendaraan Operasional",
];

export function makeAssetRegister(seed = "asset-register"): AssetRegisterRow[] {
  const rand = seedRandom(hashString(seed));
  const base = today();
  return ASSET_NAMES.map((name, i) => {
    const cost = randInt(rand, 5_000_000, 850_000_000);
    const ageMonths = randInt(rand, 2, 72);
    const monthly = Math.round(cost / 60); // ~5-year straight-line
    const accumDep = Math.min(cost, monthly * ageMonths);
    const nbv = cost - accumDep;
    const acquired = addDays(base, -ageMonths * 30);
    let status: AssetRegisterRow["status"];
    if (nbv <= 0) status = "FULLY_DEPRECIATED";
    else if (ageMonths > 60 && rand() < 0.2) status = "DISPOSED";
    else status = "ACTIVE";
    return {
      id: `ast-${pad4(7000 + i)}`,
      code: `FA-${pad4(7000 + i)}`,
      name,
      category: pickFrom(rand, ASSET_CATEGORIES),
      acquiredDate: isoDate(acquired),
      cost,
      accumDep,
      nbv,
      status,
    };
  });
}

/** Re-export of amount-formatter used by consumers. */
export const mockFormat = { fmt, isoDate, pad4 };
