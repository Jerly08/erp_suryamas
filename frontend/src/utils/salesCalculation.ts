export interface SalesCalculationItem {
  quantity?: number;
  unit_price?: number;
  discount_percent?: number;
  taxable?: boolean;
}

export interface SalesCalculationInput {
  items: SalesCalculationItem[];
  discountPercent: number;
  ppnRate: number;
  pph21Rate: number;
  pph23Rate: number;
  otherTaxAdditions: number;
  otherTaxDeductions: number;
  shippingCost: number;
  shippingTaxable: boolean;
}

export interface SalesCalculationResult {
  subtotal: number;
  discountAmount: number;
  netBeforeTax: number;
  taxableAmount: number;
  ppnAmount: number;
  pph21Amount: number;
  pph23Amount: number;
  totalDeductions: number;
  totalAmount: number;
}

const money = (value: number) => Math.round((value + Number.EPSILON) * 100) / 100;
const rate = (value: number) => Math.min(100, Math.max(0, Number(value) || 0)) / 100;

export const calculateSalesAmounts = ({
  items,
  discountPercent,
  ppnRate,
  pph21Rate,
  pph23Rate,
  otherTaxAdditions,
  otherTaxDeductions,
  shippingCost,
  shippingTaxable,
}: SalesCalculationInput): SalesCalculationResult => {
  let taxableSubtotal = 0;
  let nonTaxableSubtotal = 0;

  items.forEach((item) => {
    const gross = (Number(item.quantity) || 0) * (Number(item.unit_price) || 0);
    const lineTotal = money(gross * (1 - rate(Number(item.discount_percent) || 0)));
    if (item.taxable === false) {
      nonTaxableSubtotal += lineTotal;
    } else {
      taxableSubtotal += lineTotal;
    }
  });

  const subtotal = money(taxableSubtotal + nonTaxableSubtotal);
  const globalDiscountRate = rate(discountPercent);
  const discountAmount = money(subtotal * globalDiscountRate);
  const taxableGoods = money(taxableSubtotal * (1 - globalDiscountRate));
  const netBeforeTax = money(subtotal - discountAmount);
  const normalizedShipping = money(Math.max(0, Number(shippingCost) || 0));
  const taxableAmount = money(taxableGoods + (shippingTaxable ? normalizedShipping : 0));
  const ppnAmount = money(taxableAmount * rate(ppnRate));
  const pph21Amount = money(taxableGoods * rate(pph21Rate));
  const pph23Amount = money(taxableGoods * rate(pph23Rate));
  const totalDeductions = money(pph21Amount + pph23Amount + (Number(otherTaxDeductions) || 0));
  const totalAmount = money(
    netBeforeTax +
      normalizedShipping +
      ppnAmount +
      (Number(otherTaxAdditions) || 0) -
      totalDeductions,
  );

  return {
    subtotal,
    discountAmount,
    netBeforeTax,
    taxableAmount,
    ppnAmount,
    pph21Amount,
    pph23Amount,
    totalDeductions,
    totalAmount,
  };
};
