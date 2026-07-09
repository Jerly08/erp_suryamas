"use client";

import React, { useState, useEffect } from "react";
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  Button,
  FormControl,
  FormLabel,
  FormErrorMessage,
  Input,
  Select,
  Textarea,
  VStack,
  HStack,
  Text,
  Alert,
  AlertIcon,
  Box,
  Divider,
  useToast,
  Wrap,
  WrapItem,
  useColorModeValue,
} from "@chakra-ui/react";
import { useForm } from "react-hook-form";
import { useAuth } from "@/contexts/AuthContext";
import salesService, {
  Sale,
  SalePaymentRequest,
} from "@/services/salesService";
import cashbankService, {
  CashBank,
  getBookBalanceInIDR,
  ReferenceExchangeRate,
} from "@/services/cashbankService";
import accountService from "@/services/accountService";
import { Account as GLAccount } from "@/types/account";
import { useTranslation } from "@/hooks/useTranslation";
import { roundUsdIdrRate } from "@/utils/exchangeRate";
import {
  filterActiveLeafAccounts,
  filterOperationalCashBankAccounts,
} from "@/utils/accountFilters";

interface PaymentFormProps {
  isOpen: boolean;
  onClose: () => void;
  sale: Sale | null;
  onSave: () => void;
}

interface PaymentFormData {
  date: string;
  amount: number;
  method: string;
  reference: string;
  account_id: number;
  exchange_rate: number;
  cash_bank_id?: number;
  notes: string;
}

const PaymentForm: React.FC<PaymentFormProps> = ({
  isOpen,
  onClose,
  sale: initialSale,
  onSave,
}) => {
  const { t } = useTranslation();
  const { token, user } = useAuth();
  const [loading, setLoading] = useState(false);
  const [accounts, setAccounts] = useState<CashBank[]>([]);
  const [creditAccounts, setCreditAccounts] = useState<GLAccount[]>([]);
  const [paymentHistory, setPaymentHistory] = useState<any[]>([]);
  const [accountsLoading, setAccountsLoading] = useState(false);
  const [loadingCreditAccounts, setLoadingCreditAccounts] = useState(false);
  const [referenceRate, setReferenceRate] =
    useState<ReferenceExchangeRate | null>(null);
  const [loadingReferenceRate, setLoadingReferenceRate] = useState(false);
  const [manualRateOverride, setManualRateOverride] = useState(false);
  const [detailedSale, setDetailedSale] = useState<Sale | null>(null);
  const [loadingSaleDetail, setLoadingSaleDetail] = useState(false);
  const toast = useToast();
  const modalBg = useColorModeValue("white", "gray.800");
  const modalHeaderBg = useColorModeValue("gray.50", "gray.700");
  const modalBorderColor = useColorModeValue("gray.200", "whiteAlpha.200");
  const summaryBg = useColorModeValue("gray.50", "whiteAlpha.100");
  const allocationBg = useColorModeValue(
    "orange.50",
    "rgba(237, 137, 54, 0.12)",
  );
  const allocationBorder = useColorModeValue("orange.300", "orange.500");
  const allocationCardBg = useColorModeValue("white", "whiteAlpha.50");
  const allocationCardBorder = useColorModeValue("orange.100", "orange.500");
  const historyBg = useColorModeValue("blue.50", "rgba(66, 153, 225, 0.12)");
  const historyBorder = useColorModeValue("blue.400", "blue.500");
  const textColor = useColorModeValue("gray.700", "whiteAlpha.900");
  const headingColor = useColorModeValue("gray.800", "whiteAlpha.900");
  const mutedTextColor = useColorModeValue("gray.600", "whiteAlpha.700");
  const inputBg = useColorModeValue("white", "whiteAlpha.50");
  const inputBorderColor = useColorModeValue("gray.300", "whiteAlpha.300");
  const inputHoverBorder = useColorModeValue("gray.400", "whiteAlpha.500");
  const dividerColor = useColorModeValue("gray.200", "whiteAlpha.200");

  const {
    register,
    handleSubmit,
    reset,
    setValue,
    watch,
    formState: { errors },
  } = useForm<PaymentFormData>();

  const watchAmount = watch("amount");
  const watchAccountID = watch("account_id");
  const watchExchangeRate = watch("exchange_rate");
  const [displayAmount, setDisplayAmount] = useState("0");
  const sale = detailedSale || initialSale;
  const hasOutstandingAmount = !!sale && (sale.outstanding_amount || 0) > 0;
  const canReceivePayment =
    hasOutstandingAmount &&
    (sale?.status === "INVOICED" || sale?.status === "OVERDUE");
  const paymentBlockedMessage =
    sale && !hasOutstandingAmount
      ? t("sales.detail.paymentNoOutstanding")
      : t("sales.detail.paymentRequiresInvoice");

  const selectedAccount = accounts.find(
    (account) => account.id === watchAccountID,
  );
  const selectedAccountCurrency = (
    selectedAccount?.currency || "IDR"
  ).toUpperCase();
  const isUsdAccount = selectedAccountCurrency === "USD";
  const saleCurrency = (sale?.currency || "IDR").toUpperCase();
  const requiresExchangeRate = saleCurrency === "USD" || isUsdAccount;
  const currentBookBalanceIDR = selectedAccount
    ? getBookBalanceInIDR(selectedAccount)
    : 0;
  const appliedExchangeRate = requiresExchangeRate ? watchExchangeRate || 0 : 1;
  const incomingNativeAmount =
    requiresExchangeRate && appliedExchangeRate > 0
      ? (watchAmount || 0) / appliedExchangeRate
      : watchAmount || 0;
  const bankBookAmountIDR = watchAmount || 0;
  const endingBookBalanceIDR = currentBookBalanceIDR + bankBookAmountIDR;
  const saleHistoricalRate =
    saleCurrency === "USD" && (sale?.exchange_rate || 0) > 0
      ? sale?.exchange_rate || 1
      : 1;
  const receivableSettlementBookIDR =
    saleCurrency === "USD" && appliedExchangeRate > 0
      ? incomingNativeAmount * saleHistoricalRate
      : watchAmount || 0;
  const estimatedForexDiff = bankBookAmountIDR - receivableSettlementBookIDR;
  const estimatedForexType =
    estimatedForexDiff > 0.005
      ? "LABA KURS"
      : estimatedForexDiff < -0.005
        ? "RUGI KURS"
        : "NETRAL";
  const estimatedForexAmount = Math.abs(estimatedForexDiff);
  const schedulePreview = (() => {
    if (!sale?.payment_schedules || sale.payment_schedules.length === 0) {
      return [];
    }

    const schedules = [...sale.payment_schedules].sort(
      (left, right) => (left.sequence || 0) - (right.sequence || 0),
    );
    let remainingCurrentPaid = Math.max(sale.paid_amount || 0, 0);
    let remainingProjectedPaid = Math.max(
      (sale.paid_amount || 0) +
        Math.min(watchAmount || 0, sale.outstanding_amount || 0),
      0,
    );

    return schedules.map((schedule) => {
      const currentPaid = Math.min(remainingCurrentPaid, schedule.amount || 0);
      const projectedPaid = Math.min(
        remainingProjectedPaid,
        schedule.amount || 0,
      );
      const currentOutstanding = Math.max(
        (schedule.amount || 0) - currentPaid,
        0,
      );
      const projectedOutstanding = Math.max(
        (schedule.amount || 0) - projectedPaid,
        0,
      );
      const allocatedNow = Math.max(projectedPaid - currentPaid, 0);
      const projectedStatus =
        projectedPaid <= 0.005
          ? "PENDING"
          : projectedOutstanding <= 0.005
            ? "PAID"
            : "PARTIAL";

      remainingCurrentPaid = Math.max(remainingCurrentPaid - currentPaid, 0);
      remainingProjectedPaid = Math.max(
        remainingProjectedPaid - projectedPaid,
        0,
      );

      return {
        ...schedule,
        current_paid: currentPaid,
        current_outstanding: currentOutstanding,
        allocated_now: allocatedNow,
        projected_paid: projectedPaid,
        projected_outstanding: projectedOutstanding,
        projected_status: projectedStatus,
      };
    });
  })();
  const outstandingTermActions = schedulePreview
    .filter((schedule) => (schedule.current_outstanding || 0) > 0.005)
    .slice(0, 4)
    .map((schedule) => ({
      key: `term-${schedule.sequence}`,
      label:
        (schedule.current_paid || 0) > 0.005
          ? `Sisa Termin ${schedule.sequence}`
          : `Termin ${schedule.sequence}`,
      amount: Math.min(
        schedule.current_outstanding || 0,
        sale?.outstanding_amount || 0,
      ),
      helper: `${schedule.percentage}% invoice`,
    }));

  // Format number to Rupiah display
  const formatRupiah = (value: number | string): string => {
    const numValue = typeof value === "string" ? parseFloat(value) || 0 : value;
    return new Intl.NumberFormat("id-ID").format(numValue);
  };

  const formatAccountBalance = (amount: number, currency?: string): string => {
    const normalizedCurrency = (currency || "IDR").toUpperCase();

    if (normalizedCurrency === "USD") {
      return `USD ${(amount || 0).toLocaleString("en-US", {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      })}`;
    }

    return salesService.formatCurrency(amount || 0, normalizedCurrency);
  };

  // Parse Rupiah string to number
  const parseRupiah = (value: string): number => {
    // Remove Rp prefix, spaces, and convert Indonesian decimal format
    const cleanValue = value
      .replace(/^Rp\s*/, "") // Remove "Rp " prefix
      .replace(/\./g, "") // Remove thousand separators (dots)
      .replace(/,/, "."); // Convert comma to decimal point
    return parseFloat(cleanValue) || 0;
  };

  const applyQuickAmount = (rawAmount: number) => {
    const maxOutstanding = sale?.outstanding_amount || 0;
    const nextAmount = Math.max(0, Math.min(rawAmount || 0, maxOutstanding));
    setValue("amount", nextAmount, {
      shouldDirty: true,
      shouldValidate: true,
      shouldTouch: true,
    });
    setDisplayAmount(formatRupiah(nextAmount));
  };

  // Handle amount input change
  const handleAmountChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const inputValue = e.target.value;

    // Only allow numbers, dots, commas, spaces, and "Rp"
    const allowedCharsRegex = /^[Rp\d.,\s]*$/;
    if (!allowedCharsRegex.test(inputValue)) {
      return; // Ignore invalid characters
    }

    const numericValue = parseRupiah(inputValue);

    // Validate max amount
    if (sale && numericValue > sale.outstanding_amount) {
      return; // Don't allow amount greater than outstanding
    }

    // Update form value
    setValue("amount", numericValue);

    // Update display value
    setDisplayAmount(formatRupiah(numericValue));
  };

  useEffect(() => {
    if (!isOpen || !initialSale?.id) {
      setDetailedSale(null);
      setLoadingSaleDetail(false);
      return;
    }

    let cancelled = false;

    const loadSaleDetail = async () => {
      setLoadingSaleDetail(true);
      try {
        const response = await salesService.getSale(initialSale.id);
        if (!cancelled) {
          setDetailedSale(response);
        }
      } catch (error) {
        if (!cancelled) {
          setDetailedSale(initialSale);
        }
      } finally {
        if (!cancelled) {
          setLoadingSaleDetail(false);
        }
      }
    };

    loadSaleDetail();

    return () => {
      cancelled = true;
    };
  }, [initialSale, isOpen]);

  useEffect(() => {
    if (sale && isOpen) {
      // Set default values
      setValue("date", new Date().toISOString().split("T")[0]);
      const defaultAmount = sale.outstanding_amount || 0;
      setValue("amount", defaultAmount);
      setDisplayAmount(formatRupiah(defaultAmount));
      setValue("method", "BANK_TRANSFER");
      setValue("account_id", 0);
      setValue("exchange_rate", 1);
      setValue("reference", "");
      setValue("notes", "");

      // Load accounts for payment
      loadAccounts();
    }
  }, [sale, isOpen, setValue]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    if (!requiresExchangeRate) {
      setReferenceRate(null);
      setLoadingReferenceRate(false);
      setManualRateOverride(false);
      setValue("exchange_rate", 1);
      return;
    }

    let cancelled = false;

    const loadReferenceRate = async () => {
      setLoadingReferenceRate(true);
      try {
        const response = await cashbankService.getReferenceExchangeRate(
          "USD",
          "IDR",
        );
        if (cancelled) return;

        setReferenceRate(response);

        if (!manualRateOverride) {
          const suggestedRate =
            response?.exchange_rate ||
            (selectedAccount?.last_exchange_rate &&
            selectedAccount.last_exchange_rate > 0
              ? selectedAccount.last_exchange_rate
              : saleHistoricalRate);

          if (suggestedRate > 0) {
            setValue("exchange_rate", roundUsdIdrRate(suggestedRate, 1));
          }
        }
      } finally {
        if (!cancelled) {
          setLoadingReferenceRate(false);
        }
      }
    };

    const fallbackRate =
      selectedAccount?.last_exchange_rate &&
      selectedAccount.last_exchange_rate > 0
        ? selectedAccount.last_exchange_rate
        : saleHistoricalRate;

    if (!manualRateOverride && fallbackRate > 0 && !(watchExchangeRate > 0)) {
      setValue("exchange_rate", roundUsdIdrRate(fallbackRate, 1));
    }

    loadReferenceRate();

    return () => {
      cancelled = true;
    };
  }, [
    isOpen,
    manualRateOverride,
    requiresExchangeRate,
    saleHistoricalRate,
    selectedAccount,
    setValue,
    watchExchangeRate,
  ]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    setManualRateOverride(false);
  }, [isOpen, watchAccountID]);

  const loadAccounts = async () => {
    // Check if user is authenticated and has required permissions
    if (!token || !user) {
      toast({
        title: t("sales.messages.authRequired"),
        description: t("sales.messages.pleaseLogin"),
        status: "error",
        duration: 5000,
      });
      return;
    }

    // Check if user has permission to view accounts (based on RBAC)
    const allowedRoles = ["admin", "finance", "director", "employee"];
    if (!allowedRoles.includes(user.role.toLowerCase())) {
      toast({
        title: t("sales.messages.accessDenied"),
        description: t("messages.toast.accessDeniedDesc"),
        status: "error",
        duration: 5000,
      });
      return;
    }

    try {
      setAccountsLoading(true);

      // Use cashbank service to get payment accounts (cash and bank accounts)
      const paymentAccounts = await cashbankService.getPaymentAccounts();
      const leafPaymentAccounts = filterOperationalCashBankAccounts(paymentAccounts || []);
      setAccounts(leafPaymentAccounts);

      if (leafPaymentAccounts.length === 0) {
        toast({
          title: t("common.noData"),
          description: t("common.noData"),
          status: "warning",
          duration: 5000,
        });
      }
    } catch (error: any) {
      console.error("Error loading payment accounts:", error);

      // Set empty accounts if service fails
      setAccounts([]);

      // Provide more specific error messages based on the error
      let errorMessage = t("messages.toast.serverErrorDesc");

      if (
        error.message?.includes("403") ||
        error.message?.includes("Forbidden")
      ) {
        errorMessage = t("messages.toast.accessDeniedDesc");
      } else if (
        error.message?.includes("401") ||
        error.message?.includes("Unauthorized")
      ) {
        errorMessage = t("messages.toast.sessionExpiredDesc");
      } else if (error.message?.includes("Network")) {
        errorMessage = t("messages.toast.networkErrorDesc");
      }

      toast({
        title: t("messages.toast.error"),
        description: errorMessage,
        status: "error",
        duration: 5000,
      });
    } finally {
      setAccountsLoading(false);
    }
  };

  // Fetch credit accounts (liability) for credit card payment method selection
  const loadCreditAccounts = async () => {
    if (!token) return;
    try {
      setLoadingCreditAccounts(true);

      // Try catalog endpoint first for EMPLOYEE role, fallback to regular endpoint
      if (user?.role === "employee") {
        try {
          const catalogData = await accountService.getAccountCatalog(
            token,
            "LIABILITY",
          );
          const formattedAccounts: GLAccount[] = filterActiveLeafAccounts(catalogData).map(
            (item) => ({
              id: item.id,
              code: item.code,
              name: item.name,
              type: "LIABILITY" as const,
              is_active: item.active,
              level: 1,
              is_header: false,
              balance: 0,
              created_at: "",
              updated_at: "",
              description: "",
            }),
          );
          console.log(
            "Formatted credit accounts from catalog:",
            formattedAccounts,
          );
          setCreditAccounts(formattedAccounts);
          return; // Success, exit early
        } catch (catalogError: any) {
          console.log(
            "Catalog endpoint not available, trying regular endpoint:",
            catalogError.message,
          );
          // Fall through to try regular endpoint
        }
      }

      // Use full account data for other roles or as fallback for EMPLOYEE
      try {
        const data = await accountService.getAccounts(token, "LIABILITY");
        const list: GLAccount[] = Array.isArray(data)
          ? filterActiveLeafAccounts(data)
          : [];
        console.log("Formatted credit accounts from regular endpoint:", list);
        setCreditAccounts(list);
      } catch (regularError: any) {
        console.error("Regular accounts endpoint also failed:", regularError);
        throw regularError; // Re-throw to be caught by outer catch
      }
    } catch (err: any) {
      console.error("Error fetching credit accounts:", err);
      // If both endpoints fail, fall back to manual entry mode
      setCreditAccounts([]);

      // Only show warning for non-EMPLOYEE users or if it's not a permission error
      if (
        user?.role !== "employee" ||
        !err.message?.includes("Insufficient permissions")
      ) {
        toast({
          title: "Limited Access",
          description:
            "Unable to load credit accounts list. Credit card payment will use default liability account.",
          status: "warning",
          duration: 5000,
          isClosable: true,
        });
      }
    } finally {
      setLoadingCreditAccounts(false);
    }
  };

  const onSubmit = async (data: PaymentFormData) => {
    if (!sale) return;

    try {
      setLoading(true);

      if (!canReceivePayment) {
        toast({
          title: t("messages.toast.validationError"),
          description: paymentBlockedMessage,
          status: "warning",
          duration: 5000,
        });
        return;
      }

      // Validate required fields
      if (!data.account_id || data.account_id === 0) {
        toast({
          title: t("messages.toast.validationError"),
          description: t("purchases.payment.selectAccountRequired"),
          status: "error",
          duration: 3000,
        });
        return;
      }

      const targetAccount = accounts.find(
        (account) => account.id === data.account_id,
      );
      const targetCurrency = (targetAccount?.currency || "IDR").toUpperCase();

      if (
        requiresExchangeRate &&
        (!data.exchange_rate || data.exchange_rate <= 0)
      ) {
        toast({
          title: t("messages.toast.validationError"),
          description:
            "Kurs transaksi USD ke IDR wajib diisi sebelum menyimpan pembayaran.",
          status: "error",
          duration: 4000,
        });
        return;
      }

      // For sales payments (receivables), we don't need balance validation
      // because we're receiving money from customers, not paying out
      // Balance validation only applies to payable payments (to vendors)

      // Convert date to proper ISO datetime format for backend
      const paymentDateTime = new Date(data.date).toISOString();

      const paymentData: SalePaymentRequest = {
        payment_date: paymentDateTime, // Send full datetime in ISO format
        amount: data.amount,
        payment_method: data.method, // Use correct field name
        exchange_rate: requiresExchangeRate
          ? roundUsdIdrRate(Number(data.exchange_rate), 1)
          : 1,
        reference: data.reference || "", // Ensure it's not undefined
        cash_bank_id: data.account_id, // Use the selected account ID as cash_bank_id
        notes: data.notes || "", // Ensure it's not undefined
      };

      const response = await salesService.createIntegratedPayment(
        sale.id,
        paymentData,
      );
      const postedAccountSummary = salesService.formatCashBankSnapshotSummary(
        response.payment,
      );
      const latestStatus =
        response.updated_sale?.settlement_status ||
        response.updated_sale?.status ||
        sale.settlement_status ||
        sale.status;
      const successDescription = postedAccountSummary
        ? `Pembayaran tersimpan ke ${postedAccountSummary}. Status invoice sekarang ${salesService.getStatusLabel(latestStatus)}.`
        : `Pembayaran berhasil direkam. Status invoice sekarang ${salesService.getStatusLabel(latestStatus)}.`;

      toast({
        title: t("messages.toast.saveSuccess"),
        description: successDescription,
        status: "success",
        duration: 5000,
      });

      reset();
      onSave();
      onClose();
    } catch (error: any) {
      const responseData = error?.response?.data || {};
      const rawMessage =
        responseData.message ||
        responseData.details ||
        responseData.error ||
        error?.message ||
        "";
      const normalizedMessage = String(rawMessage).toLowerCase();
      const paymentStatusMessage =
        responseData.code === "SALE_NOT_INVOICED" ||
        responseData.code === "INVALID_SALE_STATUS" ||
        normalizedMessage.includes("must be invoiced") ||
        normalizedMessage.includes("can only be made for invoiced")
          ? t("sales.detail.paymentRequiresInvoice")
          : rawMessage || t("messages.toast.serverErrorDesc");

      toast({
        title: t("messages.toast.error"),
        description: paymentStatusMessage,
        status: "error",
        duration: 5000,
      });
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    reset();
    setDetailedSale(null);
    onClose();
  };

  const paymentMethods = [
    { value: "CASH", label: t("purchases.payment.cash") },
    { value: "BANK_TRANSFER", label: t("purchases.payment.bankTransfer") },
    { value: "CHECK", label: t("purchases.payment.check") },
    { value: "CREDIT_CARD", label: t("purchases.payment.creditCard") },
    { value: "DEBIT_CARD", label: t("purchases.payment.debitCard") },
    { value: "OTHER", label: t("common.other") },
  ];

  return (
    <Modal isOpen={isOpen} onClose={handleClose} size="lg">
      <ModalOverlay />
      <ModalContent
        bg={modalBg}
        border="1px solid"
        borderColor={modalBorderColor}
      >
        <ModalHeader
          bg={modalHeaderBg}
          borderBottomWidth="1px"
          borderColor={modalBorderColor}
          color={headingColor}
        >
          {t("purchases.payment.title")}
        </ModalHeader>
        <ModalCloseButton />

        <form onSubmit={handleSubmit(onSubmit)}>
          <ModalBody>
            {sale && (
              <Box
                mb={6}
                p={4}
                bg={summaryBg}
                borderRadius="md"
                border="1px solid"
                borderColor={modalBorderColor}
              >
                <VStack align="stretch" spacing={2}>
                  <HStack justify="space-between">
                    <Text fontSize="sm" color={mutedTextColor}>
                      {t("sales.form.code")}:
                    </Text>
                    <Text fontWeight="medium" color={textColor}>
                      {sale.code}
                    </Text>
                  </HStack>
                  <HStack justify="space-between">
                    <Text fontSize="sm" color={mutedTextColor}>
                      {t("sales.invoice")}:
                    </Text>
                    <Text fontWeight="medium" color={textColor}>
                      {sale.invoice_number || "N/A"}
                    </Text>
                  </HStack>
                  <HStack justify="space-between">
                    <Text fontSize="sm" color={mutedTextColor}>
                      {t("sales.customer")}:
                    </Text>
                    <Text fontWeight="medium" color={textColor}>
                      {sale.customer?.name || "N/A"}
                    </Text>
                  </HStack>
                  <Divider borderColor={dividerColor} />
                  <HStack justify="space-between">
                    <Text fontSize="sm" color={mutedTextColor}>
                      {t("purchases.payment.totalAmount")}:
                    </Text>
                    <Text fontWeight="bold" color={textColor}>
                      {salesService.formatCurrency(sale.total_amount)}
                    </Text>
                  </HStack>
                  <HStack justify="space-between">
                    <Text fontSize="sm" color={mutedTextColor}>
                      {t("purchases.payment.paidAmount")}:
                    </Text>
                    <Text color={textColor}>
                      {salesService.formatCurrency(sale.paid_amount)}
                    </Text>
                  </HStack>
                  <HStack justify="space-between">
                    <Text fontSize="sm" color={mutedTextColor}>
                      {t("purchases.payment.outstanding")}:
                    </Text>
                    <Text fontWeight="bold" color="orange.600">
                      {salesService.formatCurrency(sale.outstanding_amount)}
                    </Text>
                  </HStack>
                  {loadingSaleDetail && (
                    <Text fontSize="sm" color="blue.600">
                      Memuat detail termin invoice terbaru...
                    </Text>
                  )}
                </VStack>
              </Box>
            )}

            {sale && !canReceivePayment && (
              <Alert status="warning" borderRadius="md" mb={4}>
                <AlertIcon />
                <Text fontSize="sm">{paymentBlockedMessage}</Text>
              </Alert>
            )}

            {sale?.payment_schedules && sale.payment_schedules.length > 0 && (
              <Box
                mb={4}
                p={4}
                bg={allocationBg}
                borderRadius="md"
                borderLeft="4px"
                borderColor={allocationBorder}
              >
                <Text fontSize="sm" fontWeight="bold" color="orange.700" mb={2}>
                  Auto-allocation ke termin invoice
                </Text>
                <Text fontSize="sm" color={textColor} mb={3}>
                  Payment yang direkam akan otomatis masuk ke termin paling awal
                  yang masih outstanding. Jurnal AR tetap ke invoice total, tapi
                  jadwal termin di bawah ini akan ikut diperbarui.
                </Text>
                <VStack spacing={3} align="stretch">
                  {schedulePreview.map((schedule) => (
                    <Box
                      key={`${schedule.sequence}-${schedule.due_date}`}
                      p={3}
                      borderRadius="md"
                      bg={allocationCardBg}
                      border="1px solid"
                      borderColor={allocationCardBorder}
                    >
                      <HStack justify="space-between" align="start">
                        <Box>
                          <Text
                            fontSize="sm"
                            fontWeight="semibold"
                            color={headingColor}
                          >
                            Termin {schedule.sequence}
                          </Text>
                          <Text fontSize="xs" color={mutedTextColor}>
                            Jatuh tempo:{" "}
                            {salesService.formatDate(schedule.due_date)}
                          </Text>
                          {schedule.description && (
                            <Text fontSize="xs" color={mutedTextColor}>
                              {schedule.description}
                            </Text>
                          )}
                        </Box>
                        <Box textAlign="right">
                          <Text
                            fontSize="sm"
                            fontWeight="semibold"
                            color={headingColor}
                          >
                            {salesService.formatCurrency(schedule.amount)}
                          </Text>
                          <Text fontSize="xs" color={mutedTextColor}>
                            {schedule.percentage}% dari invoice
                          </Text>
                        </Box>
                      </HStack>

                      <HStack mt={2} justify="space-between" fontSize="sm">
                        <Text color={mutedTextColor}>
                          Sudah teralokasi:{" "}
                          {salesService.formatCurrency(
                            schedule.current_paid || 0,
                          )}
                        </Text>
                        <Text color={mutedTextColor}>
                          Outstanding:{" "}
                          {salesService.formatCurrency(
                            schedule.current_outstanding || 0,
                          )}
                        </Text>
                      </HStack>

                      {schedule.allocated_now > 0 && (
                        <Text fontSize="sm" color="blue.600" mt={2}>
                          Payment ini akan dialokasikan:{" "}
                          {salesService.formatCurrency(schedule.allocated_now)}
                        </Text>
                      )}

                      <Text
                        fontSize="sm"
                        mt={2}
                        color={
                          schedule.projected_status === "PAID"
                            ? "green.600"
                            : schedule.projected_status === "PARTIAL"
                              ? "orange.600"
                              : textColor
                        }
                      >
                        Status setelah simpan: {schedule.projected_status} |
                        Sisa termin:{" "}
                        {salesService.formatCurrency(
                          schedule.projected_outstanding || 0,
                        )}
                      </Text>
                    </Box>
                  ))}
                </VStack>
              </Box>
            )}

            {/* Payment History Section */}
            {sale?.sale_payments && sale.sale_payments.length > 0 && (
              <Box
                mb={4}
                p={4}
                bg={historyBg}
                borderRadius="md"
                borderLeft="4px"
                borderColor={historyBorder}
              >
                <Text fontSize="sm" fontWeight="bold" color="blue.700" mb={2}>
                  📋 {t("purchases.payment.previousPayments")}
                </Text>
                <VStack spacing={2} align="stretch">
                  {sale.sale_payments.slice(-3).map((payment, index) => (
                    <HStack key={index} justify="space-between" fontSize="sm">
                      <Text color={mutedTextColor}>
                        {salesService.formatDate(payment.date)} •{" "}
                        {payment.method}
                        {payment.reference && ` • Ref: ${payment.reference}`}
                      </Text>
                      <Text fontWeight="medium" color="green.600">
                        +{salesService.formatCurrency(payment.amount)}
                      </Text>
                    </HStack>
                  ))}
                  {sale.sale_payments.length > 3 && (
                    <Text
                      fontSize="xs"
                      color={mutedTextColor}
                      textAlign="center"
                    >
                      ...{" "}
                      {t("purchases.payment.andMorePayments", {
                        count: sale.sale_payments.length - 3,
                      })}
                    </Text>
                  )}
                </VStack>
              </Box>
            )}

            <VStack spacing={4} align="stretch">
              <HStack spacing={4}>
                <FormControl isRequired isInvalid={!!errors.date}>
                  <FormLabel>{t("purchases.payment.paymentDate")} *</FormLabel>
                  <Input
                    type="date"
                    max={new Date().toISOString().split("T")[0]} // Prevent future dates
                    bg={inputBg}
                    color={textColor}
                    borderColor={inputBorderColor}
                    _hover={{ borderColor: inputHoverBorder }}
                    {...register("date", {
                      required: t("validation.required"),
                      validate: {
                        notFuture: (value) => {
                          const today = new Date();
                          const inputDate = new Date(value);
                          return (
                            inputDate <= today ||
                            t("purchases.payment.dateCannotBeFuture")
                          );
                        },
                      },
                    })}
                  />
                  <FormErrorMessage>{errors.date?.message}</FormErrorMessage>
                </FormControl>

                <FormControl isRequired isInvalid={!!errors.amount}>
                  <FormLabel>
                    {isUsdAccount
                      ? "Nominal pelunasan (IDR) *"
                      : `${t("purchases.payment.amount")} *`}
                  </FormLabel>
                  <Box position="relative">
                    <Input
                      placeholder="Rp 0"
                      value={`Rp ${displayAmount}`}
                      textAlign="right"
                      fontWeight="medium"
                      fontSize="md"
                      pl={8}
                      bg={inputBg}
                      color={textColor}
                      borderColor={inputBorderColor}
                      _hover={{ borderColor: inputHoverBorder }}
                      {...register("amount", {
                        required: t("validation.required"),
                        min: {
                          value: 0.01,
                          message: t(
                            "purchases.payment.amountMustBeGreaterThanZero",
                          ),
                        },
                        max: {
                          value: sale?.outstanding_amount || 0,
                          message: t(
                            "purchases.payment.amountExceedsOutstanding",
                          ),
                        },
                        validate: {
                          notZero: (value) =>
                            value > 0 ||
                            t("purchases.payment.amountMustBeGreaterThanZero"),
                        },
                        onChange: handleAmountChange,
                      })}
                    />
                  </Box>

                  {/* Amount Info Display */}
                  {watchAmount > 0 && (
                    <Text fontSize="sm" color={mutedTextColor} mt={1}>
                      💰 {t("purchases.payment.payment")}:{" "}
                      <Text as="span" fontWeight="bold" color="green.600">
                        Rp {formatRupiah(watchAmount)}
                      </Text>
                      {sale && watchAmount < sale.outstanding_amount && (
                        <Text as="span" color="orange.500">
                          {" • "} {t("purchases.payment.remaining")}: Rp{" "}
                          {formatRupiah(sale.outstanding_amount - watchAmount)}
                        </Text>
                      )}
                      {sale && watchAmount === sale.outstanding_amount && (
                        <Text as="span" color="green.500">
                          {" • "} ✅ {t("purchases.payment.fullPayment")}
                        </Text>
                      )}
                    </Text>
                  )}

                  {requiresExchangeRate && (
                    <Text fontSize="sm" color="blue.600" mt={2}>
                      Piutang tetap dilunasi dalam Rupiah. Kurs transaksi di
                      bawah dipakai untuk menghitung USD native yang masuk ke
                      rekening dan nilai buku IDR yang diposting ke jurnal.
                    </Text>
                  )}

                  {/* Quick Amount Selection Buttons */}
                  {outstandingTermActions.length > 0 ? (
                    <Box mt={3}>
                      <Text fontSize="xs" color={mutedTextColor} mb={2}>
                        Shortcut sesuai termin invoice:
                      </Text>
                      <Wrap spacing={2}>
                        {outstandingTermActions.map((action) => (
                          <WrapItem key={action.key}>
                            <Button
                              size="xs"
                              variant="outline"
                              colorScheme="orange"
                              onClick={() => applyQuickAmount(action.amount)}
                              disabled={!sale?.outstanding_amount}
                              title={`${action.label} • ${salesService.formatCurrency(action.amount)}`}
                            >
                              {action.label}
                            </Button>
                          </WrapItem>
                        ))}
                        <WrapItem>
                          <Button
                            size="xs"
                            variant="solid"
                            colorScheme="blue"
                            onClick={() =>
                              applyQuickAmount(sale?.outstanding_amount || 0)
                            }
                            disabled={!sale?.outstanding_amount}
                          >
                            {t("purchases.payment.fullPayment")}
                          </Button>
                        </WrapItem>
                      </Wrap>
                      <Text fontSize="xs" color={mutedTextColor} mt={2}>
                        Klik termin untuk mengisi nominal outstanding termin itu
                        secara otomatis. Pilih full payment jika customer
                        melunasi seluruh invoice sekaligus.
                      </Text>
                    </Box>
                  ) : (
                    <HStack spacing={2} mt={2}>
                      <Button
                        size="xs"
                        variant="outline"
                        onClick={() =>
                          applyQuickAmount(
                            (sale?.outstanding_amount || 0) * 0.25,
                          )
                        }
                        disabled={!sale?.outstanding_amount}
                      >
                        25%
                      </Button>
                      <Button
                        size="xs"
                        variant="outline"
                        onClick={() =>
                          applyQuickAmount(
                            (sale?.outstanding_amount || 0) * 0.5,
                          )
                        }
                        disabled={!sale?.outstanding_amount}
                      >
                        50%
                      </Button>
                      <Button
                        size="xs"
                        variant="outline"
                        onClick={() =>
                          applyQuickAmount(
                            (sale?.outstanding_amount || 0) * 0.75,
                          )
                        }
                        disabled={!sale?.outstanding_amount}
                      >
                        75%
                      </Button>
                      <Button
                        size="xs"
                        variant="solid"
                        colorScheme="blue"
                        onClick={() =>
                          applyQuickAmount(sale?.outstanding_amount || 0)
                        }
                        disabled={!sale?.outstanding_amount}
                      >
                        {t("purchases.payment.fullPayment")}
                      </Button>
                    </HStack>
                  )}

                  <FormErrorMessage>{errors.amount?.message}</FormErrorMessage>
                  {watchAmount > (sale?.outstanding_amount || 0) && (
                    <Text fontSize="sm" color="red.500" mt={1}>
                      ⚠️{" "}
                      {t("purchases.payment.amountExceedsBalance", {
                        amount: salesService.formatCurrency(
                          sale?.outstanding_amount || 0,
                        ),
                      })}
                    </Text>
                  )}
                  {watchAmount > 0 &&
                    watchAmount <= (sale?.outstanding_amount || 0) && (
                      <Text fontSize="sm" color="green.600" mt={1}>
                        ✓ {t("purchases.payment.remainingBalance")}:{" "}
                        {salesService.formatCurrency(
                          (sale?.outstanding_amount || 0) - watchAmount,
                        )}
                      </Text>
                    )}
                  {watchAmount === (sale?.outstanding_amount || 0) &&
                    watchAmount > 0 && (
                      <Text
                        fontSize="sm"
                        color="blue.600"
                        mt={1}
                        fontWeight="medium"
                      >
                        🎉 {t("purchases.payment.willFullyPay")}
                      </Text>
                    )}
                </FormControl>
              </HStack>

              <HStack spacing={4}>
                <FormControl isRequired isInvalid={!!errors.method}>
                  <FormLabel>{t("purchases.payment.paymentMethod")}</FormLabel>
                  <Select
                    bg={inputBg}
                    color={textColor}
                    borderColor={inputBorderColor}
                    _hover={{ borderColor: inputHoverBorder }}
                    {...register("method", {
                      required: t("validation.required"),
                    })}
                  >
                    <option value="">
                      {t("purchases.payment.selectPaymentMethod")}
                    </option>
                    {paymentMethods.map((method) => (
                      <option key={method.value} value={method.value}>
                        {method.label}
                      </option>
                    ))}
                  </Select>
                  <FormErrorMessage>{errors.method?.message}</FormErrorMessage>
                </FormControl>

                {/* Cash/Bank Account dropdown for all payment methods */}
                {watch("method") && (
                  <FormControl isRequired isInvalid={!!errors.account_id}>
                    <FormLabel>
                      {watch("method") === "CASH"
                        ? t("purchases.payment.cashAccount")
                        : watch("method") === "CREDIT_CARD"
                          ? t("purchases.payment.creditCardAccount")
                          : t("purchases.payment.bankAccount")}{" "}
                      *
                    </FormLabel>
                    <Select
                      bg={inputBg}
                      color={textColor}
                      borderColor={inputBorderColor}
                      _hover={{ borderColor: inputHoverBorder }}
                      {...register("account_id", {
                        required:
                          watch("method") === "CASH"
                            ? t("purchases.payment.cashAccountRequired")
                            : watch("method") === "CREDIT_CARD"
                              ? t("purchases.payment.creditCardAccountRequired")
                              : t("purchases.payment.bankAccountRequired"),
                        setValueAs: (value) => parseInt(value) || 0,
                      })}
                      disabled={accountsLoading || accounts.length === 0}
                    >
                      {accountsLoading ? (
                        <option value="">{t("common.loading")}</option>
                      ) : accounts.length === 0 ? (
                        <option value="">
                          {t("purchases.payment.noAccountsAvailable")}
                        </option>
                      ) : (
                        <>
                          <option value="">
                            {watch("method") === "CASH"
                              ? t("purchases.payment.selectCashAccount")
                              : watch("method") === "CREDIT_CARD"
                                ? t("purchases.payment.selectCreditCardAccount")
                                : t("purchases.payment.selectBankAccount")}
                          </option>
                          {accounts
                            .filter((account) => {
                              const method = watch("method");
                              if (method === "CASH") {
                                return account.type === "CASH";
                              } else if (method === "CREDIT_CARD") {
                                // Show all accounts for credit card (could be liability or bank accounts)
                                return true;
                              } else {
                                // For other methods, show BANK accounts
                                return (
                                  account.type === "BANK" ||
                                  account.type !== "CASH"
                                );
                              }
                            })
                            .map((account) => {
                              // For sales payments, we don't need balance validation warnings
                              // since we're receiving money, not paying out
                              return (
                                <option key={account.id} value={account.id}>
                                  {account.type === "BANK" && account.bank_name
                                    ? `${account.code} - ${account.name} (${account.bank_name}) - ${formatAccountBalance(account.balance, account.currency)}${(account.frozen_amount || 0) > 0 ? ` [Terblokir: ${formatAccountBalance(account.frozen_amount, account.currency)}]` : ""}`
                                    : `${account.code} - ${account.name} (${account.type}) - ${formatAccountBalance(account.balance, account.currency)}${(account.frozen_amount || 0) > 0 ? ` [Terblokir: ${formatAccountBalance(account.frozen_amount, account.currency)}]` : ""}`}
                                </option>
                              );
                            })}
                        </>
                      )}
                    </Select>
                    <FormErrorMessage>
                      {errors.account_id?.message}
                    </FormErrorMessage>

                    {/* For sales payments, show positive account balance info */}
                    {!!watch("account_id") &&
                      (() => {
                        const selectedAccountId = watch("account_id");
                        const selectedAccount = accounts.find(
                          (account) => account.id === selectedAccountId,
                        );
                        if (!selectedAccount) return null;
                        const selectedCurrency = (
                          selectedAccount.currency || "IDR"
                        ).toUpperCase();
                        const currentAmount = watch("amount") || 0;
                        const currentBookBalance =
                          getBookBalanceInIDR(selectedAccount);
                        const exchangeRate =
                          selectedCurrency === "USD"
                            ? watchExchangeRate || 0
                            : 1;
                        const incomingNative =
                          selectedCurrency === "USD" && exchangeRate > 0
                            ? currentAmount / exchangeRate
                            : currentAmount;
                        const newBalance =
                          selectedAccount.balance + incomingNative;
                        if (currentAmount > 0) {
                          return (
                            <Text fontSize="sm" color="green.600" mt={2}>
                              💰{" "}
                              <strong>
                                {t("purchases.payment.receivingPayment")}
                              </strong>
                              <br />
                              {t("purchases.payment.currentBalance")}:{" "}
                              {formatAccountBalance(
                                selectedAccount.balance,
                                selectedAccount.currency,
                              )}{" "}
                              |{t("purchases.payment.afterPayment")}:{" "}
                              {formatAccountBalance(
                                newBalance,
                                selectedAccount.currency,
                              )}
                            </Text>
                          );
                        }

                        return (
                          <Text fontSize="sm" color="blue.600" mt={2}>
                            💰{" "}
                            <strong>
                              {t("purchases.payment.accountBalance")}:
                            </strong>{" "}
                            {formatAccountBalance(
                              selectedAccount.balance,
                              selectedAccount.currency,
                            )}
                          </Text>
                        );
                      })()}

                    {isUsdAccount && selectedAccount && watchAmount > 0 && (
                      <Box
                        mt={2}
                        p={3}
                        borderRadius="md"
                        bg={historyBg}
                        border="1px solid"
                        borderColor={historyBorder}
                      >
                        <Text
                          fontSize="sm"
                          color="blue.700"
                          fontWeight="semibold"
                        >
                          Ringkasan payment valas
                        </Text>
                        <Text fontSize="sm" color={textColor} mt={1}>
                          Piutang dilunasi:{" "}
                          {salesService.formatCurrency(watchAmount)}
                        </Text>
                        <Text fontSize="sm" color={textColor}>
                          USD native masuk:{" "}
                          {formatAccountBalance(incomingNativeAmount, "USD")}
                        </Text>
                        <Text fontSize="sm" color={textColor}>
                          Nilai buku saat ini:{" "}
                          {salesService.formatCurrency(currentBookBalanceIDR)}
                        </Text>
                        <Text fontSize="sm" color={textColor}>
                          Nilai buku setelah payment:{" "}
                          {salesService.formatCurrency(endingBookBalanceIDR)}
                        </Text>
                        <Text fontSize="sm" color="gray.700">
                          Nilai buku piutang yang ditutup:{" "}
                          {salesService.formatCurrency(
                            receivableSettlementBookIDR,
                          )}
                        </Text>
                        <Text
                          fontSize="sm"
                          color={
                            estimatedForexType === "LABA KURS"
                              ? "green.600"
                              : estimatedForexType === "RUGI KURS"
                                ? "red.600"
                                : "gray.700"
                          }
                        >
                          Estimasi {estimatedForexType}:{" "}
                          {salesService.formatCurrency(estimatedForexAmount)}
                        </Text>
                      </Box>
                    )}

                    {accounts.length === 0 && !accountsLoading && (
                      <Text fontSize="xs" color="orange.500" mt={1}>
                        ⚠️{" "}
                        {t("purchases.payment.noAccountsLoaded", {
                          type:
                            watch("method") === "CASH"
                              ? t("purchases.payment.cash").toLowerCase()
                              : watch("method") === "CREDIT_CARD"
                                ? t(
                                    "purchases.payment.creditCard",
                                  ).toLowerCase()
                                : t("purchases.payment.bank").toLowerCase(),
                        })}
                      </Text>
                    )}
                  </FormControl>
                )}
              </HStack>

              {requiresExchangeRate && (
                <Box
                  p={4}
                  borderRadius="md"
                  bg={historyBg}
                  border="1px solid"
                  borderColor={historyBorder}
                >
                  <VStack spacing={3} align="stretch">
                    <FormControl isRequired isInvalid={!!errors.exchange_rate}>
                      <FormLabel>Kurs transaksi (IDR per USD) *</FormLabel>
                      <Input
                        type="number"
                        step="1"
                        min="0"
                        placeholder="Contoh: 16880"
                        bg={inputBg}
                        color={textColor}
                        borderColor={inputBorderColor}
                        _hover={{ borderColor: inputHoverBorder }}
                        {...register("exchange_rate", {
                          required:
                            "Kurs transaksi wajib diisi untuk rekening USD.",
                          min: {
                            value: 0.000001,
                            message: "Kurs transaksi harus lebih besar dari 0.",
                          },
                          setValueAs: (value) => {
                            const parsed = Number(value);
                            return Number.isFinite(parsed)
                              ? roundUsdIdrRate(parsed, 0)
                              : undefined;
                          },
                          onChange: () => setManualRateOverride(true),
                        })}
                      />
                      <FormErrorMessage>
                        {errors.exchange_rate?.message}
                      </FormErrorMessage>
                    </FormControl>

                    {referenceRate && (
                      <Text fontSize="sm" color="blue.700">
                        Kurs referensi API: 1 USD ={" "}
                        {salesService.formatCurrency(
                          referenceRate.exchange_rate,
                        )}{" "}
                        dari {referenceRate.provider || referenceRate.source}.
                      </Text>
                    )}

                    {loadingReferenceRate && (
                      <Text fontSize="sm" color={mutedTextColor}>
                        Mengambil kurs referensi USD/IDR...
                      </Text>
                    )}

                    {watchExchangeRate > 0 && (
                      <>
                        <Text fontSize="sm" color={textColor}>
                          USD yang diterima customer payment ini:{" "}
                          {formatAccountBalance(incomingNativeAmount, "USD")}
                        </Text>
                        <Text fontSize="sm" color={textColor}>
                          Nilai buku bank yang diposting:{" "}
                          {salesService.formatCurrency(bankBookAmountIDR)}
                        </Text>
                        <Text fontSize="sm" color={textColor}>
                          Nilai buku piutang historis yang ditutup:{" "}
                          {salesService.formatCurrency(
                            receivableSettlementBookIDR,
                          )}
                        </Text>
                        {saleCurrency === "USD" && saleHistoricalRate > 0 && (
                          <Text fontSize="sm" color={textColor}>
                            Kurs invoice USD tersimpan: 1 USD ={" "}
                            {salesService.formatCurrency(saleHistoricalRate)}
                          </Text>
                        )}
                        <Text
                          fontSize="sm"
                          color={
                            estimatedForexType === "LABA KURS"
                              ? "green.600"
                              : estimatedForexType === "RUGI KURS"
                                ? "red.600"
                                : "gray.700"
                          }
                        >
                          Estimasi {estimatedForexType}:{" "}
                          {salesService.formatCurrency(estimatedForexAmount)}
                        </Text>
                        {referenceRate && manualRateOverride && (
                          <Text fontSize="sm" color="orange.600">
                            Anda sedang memakai kurs manual yang berbeda dari
                            kurs referensi API.
                          </Text>
                        )}
                      </>
                    )}
                  </VStack>
                </Box>
              )}

              <FormControl>
                <FormLabel>{t("purchases.payment.reference")}</FormLabel>
                <Input
                  placeholder={t("common.placeholders.referenceNumber")}
                  bg={inputBg}
                  color={textColor}
                  borderColor={inputBorderColor}
                  _hover={{ borderColor: inputHoverBorder }}
                  {...register("reference")}
                />
              </FormControl>

              <FormControl>
                <FormLabel>{t("purchases.payment.notes")}</FormLabel>
                <Textarea
                  placeholder={t("purchases.payment.additionalNotes")}
                  bg={inputBg}
                  color={textColor}
                  borderColor={inputBorderColor}
                  _hover={{ borderColor: inputHoverBorder }}
                  {...register("notes")}
                />
              </FormControl>
            </VStack>
          </ModalBody>

          <ModalFooter borderTopWidth="1px" borderColor={modalBorderColor}>
            <Button variant="ghost" mr={3} onClick={handleClose}>
              {t("common.cancel")}
            </Button>
            <Button
              type="submit"
              colorScheme="blue"
              isLoading={loading}
              loadingText={t("purchases.payment.processing")}
              isDisabled={loading || !canReceivePayment}
            >
              {loading
                ? t("purchases.payment.processing")
                : t("purchases.payment.recordPayment")}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  );
};

export default PaymentForm;
