import React, { useEffect, useMemo, useState } from "react";
import {
  Alert,
  AlertIcon,
  Box,
  Button,
  FormControl,
  FormLabel,
  FormHelperText,
  Input,
  VStack,
  HStack,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  IconButton,
  NumberInput,
  NumberInputField,
  Text,
  useToast,
  Select,
  FormErrorMessage,
  Textarea,
  Card,
  CardBody,
  Divider,
  InputGroup,
  InputLeftAddon,
  Checkbox,
  useColorModeValue,
} from "@chakra-ui/react";
import { useForm, useFieldArray, Controller } from "react-hook-form";
import { FiPlus, FiTrash2, FiSave } from "react-icons/fi";
import { useTranslation } from "@/hooks/useTranslation";
import { accountService } from "@/services/accountService";
import cashbankService from "@/services/cashbankService";
import {
  ssotJournalService,
  JournalAdjustmentCategory,
  SSOTJournalEntry,
} from "@/services/ssotJournalService";
import { Account } from "@/types/account";
import { useAuth } from "@/contexts/AuthContext";

// Helper function to format number with thousand separators
const formatNumberWithSeparator = (value: number | string): string => {
  if (value === "" || value === null || value === undefined) return "";
  const num =
    typeof value === "string" ? parseFormattedNumber(value) : value;
  if (isNaN(num) || num === 0) return "";
  const hasFraction = Math.abs(num % 1) > Number.EPSILON;
  return num.toLocaleString("id-ID", {
    minimumFractionDigits: hasFraction ? 2 : 0,
    maximumFractionDigits: 2,
  });
};

// Helper function to parse formatted number back to number
const parseFormattedNumber = (value: string): number => {
  if (!value || value === "") return 0;
  const sanitized = value.replace(/[^\d.,]/g, "");
  let cleaned = sanitized.replace(/[^\d]/g, "");

  if (sanitized.includes(",")) {
    const lastCommaIndex = sanitized.lastIndexOf(",");
    const integerPart = sanitized.slice(0, lastCommaIndex).replace(/[^\d]/g, "");
    const decimalPart = sanitized.slice(lastCommaIndex + 1).replace(/[^\d]/g, "").slice(0, 2);
    cleaned = `${integerPart || "0"}${decimalPart ? `.${decimalPart}` : ""}`;
  } else {
    const dotCount = (sanitized.match(/\./g) || []).length;
    const lastDotIndex = sanitized.lastIndexOf(".");
    if (dotCount === 1 && lastDotIndex >= 0 && sanitized.length - lastDotIndex - 1 <= 2) {
      const integerPart = sanitized.slice(0, lastDotIndex).replace(/[^\d]/g, "");
      const decimalPart = sanitized.slice(lastDotIndex + 1).replace(/[^\d]/g, "").slice(0, 2);
      cleaned = `${integerPart || "0"}${decimalPart ? `.${decimalPart}` : ""}`;
    }
  }

  const num = parseFloat(cleaned);
  return isNaN(num) ? 0 : num;
};

const formatCurrencyInputValue = (value: string): string => {
  const sanitized = value.replace(/[^\d.,]/g, "");
  if (!sanitized) return "";

  const hasComma = sanitized.includes(",");
  const dotCount = (sanitized.match(/\./g) || []).length;
  const lastDotIndex = sanitized.lastIndexOf(".");
  const dotDecimalCandidate =
    !hasComma &&
    dotCount === 1 &&
    lastDotIndex >= 0 &&
    sanitized.length - lastDotIndex - 1 <= 2;

  let integerDigits = sanitized.replace(/[^\d]/g, "");
  let decimalDigits = "";
  let hasDecimalSeparator = false;

  if (hasComma) {
    const lastCommaIndex = sanitized.lastIndexOf(",");
    integerDigits = sanitized.slice(0, lastCommaIndex).replace(/[^\d]/g, "");
    decimalDigits = sanitized.slice(lastCommaIndex + 1).replace(/[^\d]/g, "").slice(0, 2);
    hasDecimalSeparator = true;
  } else if (dotDecimalCandidate) {
    integerDigits = sanitized.slice(0, lastDotIndex).replace(/[^\d]/g, "");
    decimalDigits = sanitized.slice(lastDotIndex + 1).replace(/[^\d]/g, "").slice(0, 2);
    hasDecimalSeparator = true;
  }

  const integerDisplay =
    integerDigits === "" ? "0" : parseInt(integerDigits, 10).toLocaleString("id-ID");
  return `${integerDisplay}${hasDecimalSeparator ? `,${decimalDigits}` : ""}`;
};

// Currency Input Component
interface CurrencyInputProps {
  value: number;
  onChange: (value: number) => void;
  placeholder?: string;
  size?: string;
}

const CurrencyInput: React.FC<CurrencyInputProps> = ({
  value,
  onChange,
  placeholder = "0",
  size = "sm",
}) => {
  const [displayValue, setDisplayValue] = useState<string>("");
  const inputBg = useColorModeValue("white", "whiteAlpha.50");
  const inputTextColor = useColorModeValue("gray.800", "gray.100");
  const inputBorderColor = useColorModeValue("gray.300", "whiteAlpha.300");
  const inputHoverBorderColor = useColorModeValue("gray.400", "whiteAlpha.400");
  const inputFocusBorderColor = useColorModeValue("blue.500", "blue.300");
  const placeholderColor = useColorModeValue("gray.500", "gray.500");

  // Update display value when external value changes
  useEffect(() => {
    if (value && value !== 0) {
      setDisplayValue(formatNumberWithSeparator(value));
    } else {
      setDisplayValue("");
    }
  }, [value]);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const inputValue = e.target.value;

    const cleanedInput = inputValue.replace(/[^\d.,]/g, "");

    if (cleanedInput === "") {
      setDisplayValue("");
      onChange(0);
      return;
    }

    const numValue = parseFormattedNumber(cleanedInput);
    if (!isNaN(numValue)) {
      setDisplayValue(formatCurrencyInputValue(cleanedInput));
      onChange(numValue);
    }
  };

  const handleBlur = () => {
    // Re-format on blur to ensure proper display
    if (value && value !== 0) {
      setDisplayValue(formatNumberWithSeparator(value));
    }
  };

  return (
    <Input
      value={displayValue}
      onChange={handleChange}
      onBlur={handleBlur}
      inputMode="decimal"
      placeholder={placeholder}
      size={size}
      textAlign="right"
      bg={inputBg}
      color={inputTextColor}
      borderColor={inputBorderColor}
      _hover={{ borderColor: inputHoverBorderColor }}
      _focusVisible={{
        borderColor: inputFocusBorderColor,
        boxShadow: `0 0 0 1px ${inputFocusBorderColor}`,
      }}
      _placeholder={{ color: placeholderColor }}
    />
  );
};

interface JournalLineForm {
  account_id: string; // use string for select value
  description: string;
  debit: number;
  credit: number;
}

interface ManualEntryFormData {
  entry_date: string;
  reference: string;
  description: string;
  adjustment_category_code: string;
  notes: string;
  lines: JournalLineForm[];
}

type JournalEntryMode = "MANUAL" | "ADJUSTMENT";

interface ManualEntryFormProps {
  onSuccess: (entry?: SSOTJournalEntry) => void;
  onCancel: () => void;
  entryType?: JournalEntryMode;
  baseEntry?: SSOTJournalEntry;
  initialData?: {
    id: number;
    entry_number?: string;
    entry_date: string;
    reference: string;
    description: string;
    notes?: string;
    source_type?: string;
    adjustment_of?: number;
    adjustment_category_code?: string;
    adjustment_category_label?: string;
    require_director_approval?: boolean;
    adjusted_entry?: SSOTJournalEntry;
    lines?: any[];
  };
}

export default function ManualEntryForm({
  onSuccess,
  onCancel,
  entryType = "MANUAL",
  baseEntry,
  initialData,
}: ManualEntryFormProps) {
  const { t } = useTranslation();
  const toast = useToast();
  const { user } = useAuth();
  const textColor = useColorModeValue("gray.800", "gray.100");
  const mutedTextColor = useColorModeValue("gray.500", "gray.400");
  const cardBg = useColorModeValue("white", "gray.800");
  const borderColor = useColorModeValue("gray.200", "whiteAlpha.200");
  const tableHeaderBg = useColorModeValue("gray.50", "whiteAlpha.100");
  const tableRowHoverBg = useColorModeValue("gray.50", "whiteAlpha.100");
  const summaryBg = useColorModeValue("gray.50", "whiteAlpha.100");
  const inputBg = useColorModeValue("white", "whiteAlpha.50");
  const inputBorderColor = useColorModeValue("gray.300", "whiteAlpha.300");
  const inputHoverBorderColor = useColorModeValue("gray.400", "whiteAlpha.400");
  const inputFocusBorderColor = useColorModeValue("blue.500", "blue.300");
  const placeholderColor = useColorModeValue("gray.500", "gray.500");
  const fieldStyles = {
    bg: inputBg,
    color: textColor,
    borderColor: inputBorderColor,
    _hover: { borderColor: inputHoverBorderColor },
    _focusVisible: {
      borderColor: inputFocusBorderColor,
      boxShadow: `0 0 0 1px ${inputFocusBorderColor}`,
    },
    _placeholder: { color: placeholderColor },
  };
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [cashBankLinkedAccounts, setCashBankLinkedAccounts] = useState<
    Account[]
  >([]);
  const [adjustmentCategories, setAdjustmentCategories] = useState<
    JournalAdjustmentCategory[]
  >([]);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const normalizedRole = String(user?.role || "").toLowerCase();
  const effectiveEntryType: JournalEntryMode =
    initialData?.source_type === "ADJUSTMENT" ? "ADJUSTMENT" : entryType;
  const isAdjustmentEntry = effectiveEntryType === "ADJUSTMENT";
  const isEmployeeJournalSubmitter = normalizedRole === "employee";
  const isFinanceStaffJournalSubmitter =
    normalizedRole === "finance_staff" || normalizedRole === "accounting";
  const isFinanceManagerJournalSubmitter =
    normalizedRole === "finance_manager" || normalizedRole === "finance";
  const [requireDirectorApproval, setRequireDirectorApproval] = useState(
    Boolean(initialData?.require_director_approval),
  );
  const relatedOriginalEntry =
    baseEntry ||
    initialData?.adjusted_entry ||
    (initialData?.adjustment_of
      ? ({
          id: initialData.adjustment_of,
          entry_number: `JE-${initialData.adjustment_of}`,
          source_type: "MANUAL",
          entry_date: initialData.entry_date,
          description: "",
          total_debit: 0,
          total_credit: 0,
          status: "POSTED",
          is_balanced: true,
          is_auto_generated: false,
          created_by: 0,
          created_at: initialData.entry_date,
          updated_at: initialData.entry_date,
        } as SSOTJournalEntry)
      : undefined);
  const effectiveAdjustmentOf =
    relatedOriginalEntry?.id || initialData?.adjustment_of;
  const defaultReference =
    isAdjustmentEntry && baseEntry && !initialData
      ? `ADJ-${baseEntry.entry_number}`
      : initialData?.reference || "";
  const defaultDescription =
    isAdjustmentEntry && baseEntry && !initialData
      ? `Adjustment of ${baseEntry.entry_number}`
      : initialData?.description || "";

  const {
    register,
    control,
    handleSubmit,
    watch,
    setValue,
    formState: { errors },
  } = useForm<ManualEntryFormData>({
    defaultValues: {
      entry_date: initialData
        ? initialData.entry_date.split("T")[0]
        : new Date().toISOString().split("T")[0],
      reference: defaultReference,
      description: defaultDescription,
      adjustment_category_code:
        initialData?.adjustment_category_code ||
        baseEntry?.adjustment_requirement_code ||
        "",
      notes: initialData?.notes || "",
      lines: initialData?.lines
        ? initialData.lines.map((l) => ({
            account_id: l.account_id.toString(),
            description: l.description,
            debit: l.debit_amount,
            credit: l.credit_amount,
          }))
        : [
            { account_id: "", description: "", debit: 0, credit: 0 },
            { account_id: "", description: "", debit: 0, credit: 0 },
          ],
    },
  });

  useEffect(() => {
    setRequireDirectorApproval(Boolean(initialData?.require_director_approval));
  }, [initialData?.id, initialData?.require_director_approval]);

  const { fields, append, remove } = useFieldArray({
    control,
    name: "lines",
  });

  // Load accounts
  useEffect(() => {
    const loadAccounts = async () => {
      setLoading(true);
      try {
        // Get account hierarchy or list
        // We can use getAccountCatalog for faster loading if available, but getAccounts gives full info
        // Using getAccounts to ensure we have code and name
        const token = localStorage.getItem("token") || "";
        if (!token) return;

        const [loadedAccounts, linkedCashBankAccounts] = await Promise.all([
          accountService.getAccounts(token),
          cashbankService
            .getCashBankAccounts({ includeHolding: true })
            .catch((error) => {
              console.warn(
                "Failed to load cash/bank accounts for journal filtering:",
                error,
              );
              return [];
            }),
        ]);
        const activeLeafAccounts = loadedAccounts
          .filter((account) => account.is_active && !account.is_header)
          .sort((a, b) => a.code.localeCompare(b.code));
        const cashBankLinkedAccountIDs = new Set(
          linkedCashBankAccounts
            .filter(
              (account) =>
                account.is_active &&
                typeof account.account_id === "number" &&
                account.account_id > 0,
            )
            .map((account) => account.account_id),
        );
        const selectableAccounts = activeLeafAccounts.filter(
          (account) => !cashBankLinkedAccountIDs.has(account.id),
        );
        const blockedAccounts = activeLeafAccounts.filter((account) =>
          cashBankLinkedAccountIDs.has(account.id),
        );
        setAccounts(selectableAccounts);
        setCashBankLinkedAccounts(blockedAccounts);
      } catch (error) {
        console.error("Failed to load accounts", error);
        toast({
          title: t("common.error"),
          description: "Failed to load accounts",
          status: "error",
          duration: 3000,
        });
      } finally {
        setLoading(false);
      }
    };
    loadAccounts();
  }, [toast, t]);

  useEffect(() => {
    if (!isAdjustmentEntry) {
      setAdjustmentCategories([]);
      return;
    }

    const loadAdjustmentCategories = async () => {
      try {
        const categories = await ssotJournalService.getAdjustmentCategories();
        setAdjustmentCategories(categories);
      } catch (error) {
        console.error("Failed to load journal adjustment categories", error);
        toast({
          title: t("common.error"),
          description: "Gagal memuat kategori jurnal adjustment",
          status: "error",
          duration: 3000,
        });
      }
    };

    loadAdjustmentCategories();
  }, [isAdjustmentEntry, toast, t]);

  const watchLines = watch("lines");
  const selectedAdjustmentCategoryCode = watch("adjustment_category_code");
  const selectedAdjustmentCategory = useMemo(
    () =>
      adjustmentCategories.find(
        (category) => category.code === selectedAdjustmentCategoryCode,
      ),
    [adjustmentCategories, selectedAdjustmentCategoryCode],
  );
  const cashBankLinkedAccountById = useMemo(
    () =>
      new Map(
        cashBankLinkedAccounts.map((account) => [account.id, account] as const),
      ),
    [cashBankLinkedAccounts],
  );
  const blockedSelectedAccounts = useMemo(
    () =>
      watchLines
        .map((line, index) => {
          const account = cashBankLinkedAccountById.get(
            Number(line.account_id || 0),
          );
          if (!account) {
            return null;
          }

          return {
            lineNumber: index + 1,
            account,
          };
        })
        .filter(
          (
            item,
          ): item is {
            lineNumber: number;
            account: Account;
          } => item !== null,
        ),
    [cashBankLinkedAccountById, watchLines],
  );
  const hasBlockedCashBankSelection = blockedSelectedAccounts.length > 0;

  const totalDebit = watchLines.reduce(
    (sum, line) => sum + (Number(line.debit) || 0),
    0,
  );
  const totalCredit = watchLines.reduce(
    (sum, line) => sum + (Number(line.credit) || 0),
    0,
  );
  const isBalanced = Math.abs(totalDebit - totalCredit) < 0.01;

  const isSubmittingRef = React.useRef(false);

  const onSubmit = async (data: ManualEntryFormData) => {
    if (isSubmittingRef.current) return;

    if (!isBalanced) {
      toast({
        title: t("common.error"),
        description: t("journal.validation.unbalanced"),
        status: "error",
        duration: 3000,
      });
      return;
    }

    if (totalDebit === 0) {
      toast({
        title: t("common.error"),
        description: t("journal.validation.zero_amount"),
        status: "error",
        duration: 3000,
      });
      return;
    }

    if (isAdjustmentEntry && !data.adjustment_category_code) {
      toast({
        title: t("common.error"),
        description:
          "Kategori transaksi adjustment wajib dipilih agar kebutuhan adjustment client terdokumentasi.",
        status: "error",
        duration: 4000,
      });
      return;
    }

    if (hasBlockedCashBankSelection) {
      const blockedAccountSummary = blockedSelectedAccounts
        .map(
          ({ lineNumber, account }) =>
            `baris ${lineNumber} (${account.code} - ${account.name})`,
        )
        .join(", ");

      toast({
        title: t("common.error"),
        description: `Akun Cash & Bank terhubung tidak bisa dipakai di jurnal manual: ${blockedAccountSummary}. Gunakan menu Deposit, Withdrawal, atau Transfer.`,
        status: "error",
        duration: 6000,
      });
      return;
    }

    isSubmittingRef.current = true;
    setSubmitting(true);
    try {
      const formattedLines = data.lines
        .map((line) => ({
          account_id: parseInt(line.account_id),
          description: line.description || data.description, // Use line desc or fallback to header desc
          debit_amount: Number(line.debit) || 0,
          credit_amount: Number(line.credit) || 0,
        }))
        .filter(
          (line) =>
            line.account_id &&
            (line.debit_amount > 0 || line.credit_amount > 0),
        );

      if (formattedLines.length < 2) {
        toast({
          title: t("common.error"),
          description: t("journal.validation.min_lines"),
          status: "error",
          duration: 3000,
        });
        isSubmittingRef.current = false;
        setSubmitting(false);
        return;
      }

      const payload = {
        entry_date: new Date(data.entry_date).toISOString(),
        description: data.description,
        reference: data.reference,
        notes: data.notes,
        source_type: effectiveEntryType,
        adjustment_of: effectiveAdjustmentOf,
        adjustment_category_code: isAdjustmentEntry
          ? data.adjustment_category_code
          : undefined,
        require_director_approval:
          isFinanceManagerJournalSubmitter && !initialData?.id
            ? requireDirectorApproval
            : false,
        requested_by_finance_manager:
          !initialData?.id && isFinanceManagerJournalSubmitter,
        auto_post:
          !initialData?.id &&
          isFinanceManagerJournalSubmitter &&
          !requireDirectorApproval,
        lines: formattedLines,
      };

      let savedEntry: SSOTJournalEntry;
      if (initialData?.id) {
        savedEntry = await ssotJournalService.updateJournalEntry(
          initialData.id,
          payload,
        );
        toast({
          title: t("common.success"),
          description: t("journal.update_success"),
          status: "success",
          duration: 3000,
        });
      } else {
        savedEntry = await ssotJournalService.createJournalEntry(payload);
        toast({
          title: t("common.success"),
          description:
            savedEntry?.status === "DRAFT" &&
            savedEntry?.approval_stage === "DIRECTOR"
              ? "Draft jurnal berhasil dikirim ke director untuk approval final."
              : savedEntry?.status === "POSTED" &&
                  isFinanceManagerJournalSubmitter
                ? "Jurnal berhasil diposting oleh finance manager."
                : isAdjustmentEntry && isFinanceStaffJournalSubmitter
                  ? t("journal.adjustmentCreateSuccess")
                  : isEmployeeJournalSubmitter
                    ? "Draft jurnal berhasil disimpan. Finance akan memverifikasi dan memposting jurnal ini."
                    : t("journal.create_success"),
          status: "success",
          duration: 3000,
        });
      }

      onSuccess(savedEntry);
    } catch (error: any) {
      console.error("Submit error:", error);
      const message =
        typeof error?.message === "string" &&
        error.message.includes("linked to Cash & Bank")
          ? "Akun kas/bank yang terhubung ke modul Cash & Bank tidak bisa dipakai di jurnal manual. Gunakan menu Deposit, Withdrawal, atau Transfer agar saldo tetap sinkron."
          : error.message || t("journal.create_error");
      toast({
        title: t("common.error"),
        description: message,
        status: "error",
        duration: 3000,
      });
      isSubmittingRef.current = false;
    } finally {
      setSubmitting(false);
      isSubmittingRef.current = false;
    }
  };

  const formatCurrency = (val: number) => {
    return new Intl.NumberFormat("id-ID", {
      style: "currency",
      currency: "IDR",
      minimumFractionDigits: 0,
    }).format(val);
  };

  return (
    <Box as="form" onSubmit={handleSubmit(onSubmit)}>
      <VStack spacing={6} align="stretch">
        {isAdjustmentEntry && (
          <Alert status="info" borderRadius="md">
            <AlertIcon />
            <Box>
              <Text>{t("journal.adjustmentDraftInfo")}</Text>
              {relatedOriginalEntry && (
                <Text fontSize="sm" mt={1}>
                  {t("journal.adjustmentSourceInfo", {
                    entryNumber: relatedOriginalEntry.entry_number,
                  })}
                </Text>
              )}
            </Box>
          </Alert>
        )}
        {isEmployeeJournalSubmitter && (
          <Alert status="info" borderRadius="md">
            <AlertIcon />
            Jurnal manual dari role employee selalu disimpan sebagai draft dan
            akan diverifikasi oleh finance sebelum diposting.
          </Alert>
        )}
        <Alert
          status={hasBlockedCashBankSelection ? "warning" : "info"}
          borderRadius="md"
          variant="left-accent"
        >
          <AlertIcon />
          <Box>
            <Text fontWeight="semibold">
              Akun Cash & Bank tidak tersedia di jurnal manual.
            </Text>
            <Text fontSize="sm">
              Untuk menjaga saldo buku besar dan modul Cash & Bank tetap
              sinkron, transaksi kas/bank harus dicatat lewat menu Deposit,
              Withdrawal, atau Transfer.
            </Text>
            {hasBlockedCashBankSelection && (
              <Text fontSize="sm" mt={1}>
                Draft ini masih memakai akun terhubung di{" "}
                {blockedSelectedAccounts
                  .map(
                    ({ lineNumber, account }) =>
                      `baris ${lineNumber} (${account.code})`,
                  )
                  .join(", ")}
                . Ganti akun tersebut sebelum menyimpan.
              </Text>
            )}
          </Box>
        </Alert>

        <Card variant="outline" bg={cardBg} borderColor={borderColor}>
          <CardBody>
            <VStack spacing={4}>
              <HStack w="full" spacing={4}>
                <FormControl isRequired isInvalid={!!errors.entry_date}>
                  <FormLabel>Date</FormLabel>
                  <Input
                    type="date"
                    {...register("entry_date", { required: true })}
                    {...fieldStyles}
                  />
                  <FormErrorMessage>
                    {t("journal.validation.date_required")}
                  </FormErrorMessage>
                </FormControl>
                <FormControl isRequired isInvalid={!!errors.reference}>
                  <FormLabel>Reference</FormLabel>
                  <Input
                    placeholder={
                      isAdjustmentEntry ? "e.g. ADJ-001" : "e.g. JE-001"
                    }
                    {...register("reference", { required: true })}
                    {...fieldStyles}
                  />
                  <FormErrorMessage>
                    {t("journal.validation.reference_required")}
                  </FormErrorMessage>
                </FormControl>
              </HStack>
              <FormControl isRequired isInvalid={!!errors.description}>
                <FormLabel>Description</FormLabel>
                <Input
                  placeholder={
                    isAdjustmentEntry
                      ? "Deskripsi jurnal adjustment"
                      : "Journal description"
                  }
                  {...register("description", { required: true })}
                  {...fieldStyles}
                />
                <FormErrorMessage>
                  {t("journal.validation.description_required")}
                </FormErrorMessage>
              </FormControl>
              {isAdjustmentEntry && (
                <FormControl
                  isRequired
                  isInvalid={!!errors.adjustment_category_code}
                >
                  <FormLabel>Kategori Transaksi Adjustment</FormLabel>
                  <Select
                    placeholder="Pilih kategori transaksi"
                    {...register("adjustment_category_code", {
                      required: isAdjustmentEntry,
                    })}
                    {...fieldStyles}
                  >
                    {adjustmentCategories.map((category) => (
                      <option key={category.code} value={category.code}>
                        {category.label}
                      </option>
                    ))}
                  </Select>
                  <FormHelperText>
                    Mapping ini membantu finance menentukan jenis transaksi
                    client yang memang membutuhkan jurnal adjustment.
                  </FormHelperText>
                  <FormErrorMessage>
                    Kategori transaksi adjustment wajib dipilih.
                  </FormErrorMessage>
                  {selectedAdjustmentCategory && (
                    <Box
                      mt={3}
                      p={3}
                      bg="orange.50"
                      borderWidth="1px"
                      borderColor="orange.100"
                      borderRadius="md"
                    >
                      <Text fontWeight="semibold" color="orange.700">
                        {selectedAdjustmentCategory.label}
                      </Text>
                      <Text fontSize="sm" color="orange.800" mt={1}>
                        {selectedAdjustmentCategory.description}
                      </Text>
                      {selectedAdjustmentCategory.coa && (
                        <Text fontSize="xs" color="orange.700" mt={2}>
                          COA: {selectedAdjustmentCategory.coa}
                        </Text>
                      )}
                      {selectedAdjustmentCategory.examples.length > 0 && (
                        <Text fontSize="xs" color="orange.700" mt={2}>
                          Contoh:{" "}
                          {selectedAdjustmentCategory.examples.join("; ")}
                        </Text>
                      )}
                    </Box>
                  )}
                </FormControl>
              )}
              <FormControl>
                <FormLabel>Notes</FormLabel>
                <Textarea
                  placeholder="Optional notes"
                  {...register("notes")}
                  {...fieldStyles}
                />
              </FormControl>
              {isFinanceManagerJournalSubmitter && !initialData?.id && (
                <FormControl>
                  <Checkbox
                    isChecked={requireDirectorApproval}
                    onChange={(event) =>
                      setRequireDirectorApproval(event.target.checked)
                    }
                    colorScheme="blue"
                  >
                    Minta approval Director
                  </Checkbox>
                  <FormHelperText>
                    Jika dicentang, jurnal akan disimpan sebagai draft approval
                    director. Jika tidak, finance manager dapat langsung
                    memposting jurnal ini.
                  </FormHelperText>
                </FormControl>
              )}
            </VStack>
          </CardBody>
        </Card>

        <Card variant="outline" bg={cardBg} borderColor={borderColor}>
          <CardBody>
            <Table size="sm" color={textColor}>
              <Thead bg={tableHeaderBg}>
                <Tr>
                  <Th width="30%" color={mutedTextColor}>
                    Account
                  </Th>
                  <Th width="25%" color={mutedTextColor}>
                    Description (Optional)
                  </Th>
                  <Th width="20%" color={mutedTextColor}>
                    Debit
                  </Th>
                  <Th width="20%" color={mutedTextColor}>
                    Credit
                  </Th>
                  <Th width="5%" color={mutedTextColor}></Th>
                </Tr>
              </Thead>
              <Tbody>
                {fields.map((field, index) => (
                  <Tr key={field.id} _hover={{ bg: tableRowHoverBg }}>
                    <Td>
                      <Controller
                        control={control}
                        name={`lines.${index}.account_id`}
                        rules={{ required: true }}
                        render={({ field }) => {
                          const selectedLinkedAccount =
                            cashBankLinkedAccountById.get(
                              Number(field.value || 0),
                            );

                          return (
                            <>
                              <Select
                                {...field}
                                placeholder={
                                  loading
                                    ? "Loading accounts..."
                                    : "Select Account"
                                }
                                size="sm"
                                {...fieldStyles}
                              >
                                {selectedLinkedAccount && (
                                  <option
                                    value={selectedLinkedAccount.id}
                                    disabled
                                  >
                                    {selectedLinkedAccount.code} -{" "}
                                    {selectedLinkedAccount.name} (Gunakan modul
                                    Cash & Bank)
                                  </option>
                                )}
                                {accounts.map((acc) => (
                                  <option key={acc.id} value={acc.id}>
                                    {acc.code} - {acc.name}
                                  </option>
                                ))}
                              </Select>
                              {selectedLinkedAccount && (
                                <Text fontSize="xs" color="orange.500" mt={1}>
                                  Akun ini terhubung ke modul Cash & Bank. Pilih
                                  akun lain atau catat transaksi lewat Deposit,
                                  Withdrawal, atau Transfer.
                                </Text>
                              )}
                            </>
                          );
                        }}
                      />
                    </Td>
                    <Td>
                      <Input
                        size="sm"
                        {...register(`lines.${index}.description`)}
                        placeholder="Line description"
                        {...fieldStyles}
                      />
                    </Td>
                    <Td>
                      <Controller
                        control={control}
                        name={`lines.${index}.debit`}
                        render={({ field }) => (
                          <CurrencyInput
                            value={field.value || 0}
                            onChange={(val) => field.onChange(val)}
                            placeholder="0"
                            size="sm"
                          />
                        )}
                      />
                    </Td>
                    <Td>
                      <Controller
                        control={control}
                        name={`lines.${index}.credit`}
                        render={({ field }) => (
                          <CurrencyInput
                            value={field.value || 0}
                            onChange={(val) => field.onChange(val)}
                            placeholder="0"
                            size="sm"
                          />
                        )}
                      />
                    </Td>
                    <Td>
                      <IconButton
                        aria-label="Remove line"
                        icon={<FiTrash2 />}
                        size="xs"
                        colorScheme="red"
                        variant="ghost"
                        onClick={() => remove(index)}
                      />
                    </Td>
                  </Tr>
                ))}
              </Tbody>
            </Table>
            <Button
              leftIcon={<FiPlus />}
              size="sm"
              variant="ghost"
              mt={2}
              onClick={() =>
                append({ account_id: "", description: "", debit: 0, credit: 0 })
              }
            >
              Add Line
            </Button>
          </CardBody>
        </Card>

        {/* Totals Section */}
        <Box
          p={4}
          bg={summaryBg}
          borderRadius="md"
          borderWidth="1px"
          borderColor={borderColor}
        >
          <HStack justify="space-between" align="center">
            <Text fontWeight="bold" color={textColor}>
              Total
            </Text>
            <HStack spacing={8}>
              <VStack align="end" spacing={0}>
                <Text fontSize="xs" color={mutedTextColor}>
                  Total Debit
                </Text>
                <Text fontWeight="bold" color="green.600">
                  {formatCurrency(totalDebit)}
                </Text>
              </VStack>
              <VStack align="end" spacing={0}>
                <Text fontSize="xs" color={mutedTextColor}>
                  Total Credit
                </Text>
                <Text fontWeight="bold" color="red.600">
                  {formatCurrency(totalCredit)}
                </Text>
              </VStack>
              <VStack align="end" spacing={0}>
                <Text fontSize="xs" color={mutedTextColor}>
                  Difference
                </Text>
                <Text
                  fontWeight="bold"
                  color={isBalanced ? "green.500" : "red.500"}
                >
                  {formatCurrency(Math.abs(totalDebit - totalCredit))}
                </Text>
              </VStack>
            </HStack>
          </HStack>
        </Box>

        <HStack justify="flex-end" spacing={4}>
          <Button onClick={onCancel}>Cancel</Button>
          <Button
            colorScheme="blue"
            leftIcon={<FiSave />}
            type="submit"
            isLoading={submitting}
            isDisabled={
              !isBalanced ||
              totalDebit === 0 ||
              hasBlockedCashBankSelection ||
              (isAdjustmentEntry && !selectedAdjustmentCategoryCode)
            }
          >
            {initialData
              ? isAdjustmentEntry
                ? "Simpan Perubahan Adjustment"
                : "Simpan Perubahan Draft"
              : isAdjustmentEntry && isFinanceStaffJournalSubmitter
                ? "Simpan Draft Adjustment"
                : isFinanceManagerJournalSubmitter && requireDirectorApproval
                  ? "Kirim ke Director"
                  : isFinanceManagerJournalSubmitter
                    ? "Post Journal"
                    : isEmployeeJournalSubmitter
                      ? "Simpan Draft"
                      : isAdjustmentEntry
                        ? "Simpan Adjustment"
                        : "Save Journal"}
          </Button>
        </HStack>
      </VStack>
    </Box>
  );
}
