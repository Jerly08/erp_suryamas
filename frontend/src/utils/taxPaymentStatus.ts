import type { Payment } from "@/services/paymentService";
import type { Purchase } from "@/services/purchaseService";
import type { Sale } from "@/services/salesService";

export type TaxPaymentStatus = "NONE" | "PAID" | "UNPAID" | "PARTIAL_PAID";

export interface TaxPaymentStatusMeta {
  status: TaxPaymentStatus;
  label: string;
  colorScheme: "gray" | "green" | "orange" | "red";
  totalAmount: number;
  paidAmount: number;
  outstandingAmount: number;
  description: string;
}

const MONEY_EPSILON = 0.005;

const statusLabels: Record<TaxPaymentStatus, string> = {
  NONE: "Tidak Ada PPh",
  PAID: "Paid",
  UNPAID: "Unpaid",
  PARTIAL_PAID: "Partial Paid",
};

const statusColors: Record<
  TaxPaymentStatus,
  TaxPaymentStatusMeta["colorScheme"]
> = {
  NONE: "gray",
  PAID: "green",
  UNPAID: "red",
  PARTIAL_PAID: "orange",
};

const buildTaxPaymentStatusMeta = (
  totalAmount: number,
  paidAmount: number,
  noTaxDescription = "Transaksi ini tidak memiliki komponen PPh.",
): TaxPaymentStatusMeta => {
  const total = Math.max(totalAmount || 0, 0);
  const paid = Math.max(Math.min(paidAmount || 0, total), 0);
  const outstanding = Math.max(total - paid, 0);

  let status: TaxPaymentStatus = "NONE";
  if (total > MONEY_EPSILON) {
    if (paid >= total - MONEY_EPSILON) {
      status = "PAID";
    } else if (paid > MONEY_EPSILON) {
      status = "PARTIAL_PAID";
    } else {
      status = "UNPAID";
    }
  }

  const description =
    status === "NONE"
      ? noTaxDescription
      : status === "PAID"
        ? "Seluruh PPh sudah tercatat sebagai dibayar/settled."
        : status === "PARTIAL_PAID"
          ? "Sebagian PPh sudah tercatat, masih ada sisa PPh yang perlu dimonitor."
          : "PPh sudah terbentuk, namun belum ada pembayaran/settlement PPh yang tercatat.";

  return {
    status,
    label: statusLabels[status],
    colorScheme: statusColors[status],
    totalAmount: total,
    paidAmount: paid,
    outstandingAmount: outstanding,
    description,
  };
};

export const getPaymentPPhStatus = (payment: Payment): TaxPaymentStatusMeta => {
  const pphAmount = payment.pph_amount || 0;

  const paymentAny = payment as Payment & {
    pph_paid_amount?: number;
    pph_payment_status?: TaxPaymentStatus;
  };

  let paidAmount = paymentAny.pph_paid_amount || 0;
  if (
    pphAmount > MONEY_EPSILON &&
    (payment.payment_type?.startsWith("TAX_PPH") ||
      payment.code?.startsWith("SETOR-PPH"))
  ) {
    paidAmount = pphAmount;
  }

  if (paymentAny.pph_payment_status === "PAID") {
    paidAmount = pphAmount;
  } else if (paymentAny.pph_payment_status === "UNPAID") {
    paidAmount = 0;
  }

  return buildTaxPaymentStatusMeta(
    pphAmount,
    paidAmount,
    "Payment ini tidak memiliki potongan PPh.",
  );
};

export const getSalePPhStatus = (sale: Sale): TaxPaymentStatusMeta => {
  const legacyPPh = sale.pph || 0;
  const pph21Amount = sale.pph21_amount || 0;
  const pph23Amount = sale.pph23_amount || 0;
  const totalPPh = legacyPPh + pph21Amount + pph23Amount;

  const saleAny = sale as Sale & {
    pph_paid_amount?: number;
    pph_payment_status?: TaxPaymentStatus;
  };

  let paidAmount = saleAny.pph_paid_amount || 0;
  if (sale.pph23_settlement_date && pph23Amount > MONEY_EPSILON) {
    paidAmount += pph23Amount;
  }
  if (saleAny.pph_payment_status === "PAID") {
    paidAmount = totalPPh;
  } else if (saleAny.pph_payment_status === "UNPAID") {
    paidAmount = 0;
  }

  return buildTaxPaymentStatusMeta(
    totalPPh,
    paidAmount,
    "Sales invoice ini tidak memiliki komponen PPh.",
  );
};

export const getPurchasePPhStatus = (
  purchase: Purchase,
): TaxPaymentStatusMeta => {
  const pph21Amount = purchase.pph21_amount || 0;
  const pph23Amount = purchase.pph23_amount || 0;
  const totalPPh = pph21Amount + pph23Amount;

  const purchaseAny = purchase as Purchase & {
    pph_paid_amount?: number;
    pph_payment_status?: TaxPaymentStatus;
    pph_payment_date?: string;
    pph_settlement_date?: string;
  };

  let paidAmount = purchaseAny.pph_paid_amount || 0;
  if (purchaseAny.pph_payment_status === "PAID") {
    paidAmount = totalPPh;
  } else if (purchaseAny.pph_payment_status === "PARTIAL_PAID") {
    paidAmount = purchaseAny.pph_paid_amount || 0;
  } else if (purchaseAny.pph_payment_status === "UNPAID") {
    paidAmount = 0;
  } else if (
    !purchaseAny.pph_payment_status &&
    totalPPh > MONEY_EPSILON &&
    (purchaseAny.pph_payment_date || purchaseAny.pph_settlement_date)
  ) {
    paidAmount = totalPPh;
  }

  return buildTaxPaymentStatusMeta(
    totalPPh,
    paidAmount,
    "Purchase ini tidak memiliki komponen PPh.",
  );
};
