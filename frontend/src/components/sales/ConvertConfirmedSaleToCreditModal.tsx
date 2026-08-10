"use client";

import React, { useEffect, useMemo, useState } from "react";
import {
  Alert,
  AlertIcon,
  Box,
  Button,
  FormControl,
  FormHelperText,
  FormLabel,
  HStack,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Select,
  Spinner,
  Text,
  useToast,
  VStack,
} from "@chakra-ui/react";
import { FiCreditCard } from "react-icons/fi";
import { useAuth } from "@/contexts/AuthContext";
import accountService from "@/services/accountService";
import salesService, {
  type Sale,
} from "@/services/salesService";
import type { AccountCatalogItem } from "@/types/account";
import SearchableSelect from "@/components/common/SearchableSelect";

interface ConvertConfirmedSaleToCreditModalProps {
  isOpen: boolean;
  onClose: () => void;
  sale: Sale | null;
  onSuccess: () => void;
}

const paymentTerms = ["NET_15", "NET_30", "NET_60", "NET_90"] as const;

const toDateInputValue = (value?: string): string => {
  if (!value) return "";
  return value.slice(0, 10);
};

const addDaysToDate = (dateValue: string, days: number): string => {
  const date = new Date(`${dateValue}T12:00:00`);
  if (Number.isNaN(date.getTime())) return "";
  date.setDate(date.getDate() + days);
  const year = date.getFullYear();
  const month = `${date.getMonth() + 1}`.padStart(2, "0");
  const day = `${date.getDate()}`.padStart(2, "0");
  return `${year}-${month}-${day}`;
};

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null;

const getApiErrorMessage = (error: unknown): string | undefined => {
  if (!isRecord(error)) return undefined;

  const response = error.response;
  if (!isRecord(response) || !("data" in response)) {
    return undefined;
  }

  const data = response.data;
  if (!isRecord(data)) return undefined;

  const context = data.context;
  if (isRecord(context) && typeof context.details === "string") {
    return context.details;
  }

  for (const key of ["message", "error"]) {
    if (typeof data[key] === "string") {
      return data[key];
    }
  }

  return undefined;
};

const isReceivableAccount = (account: AccountCatalogItem): boolean => {
  if (!account.active || account.is_header) return false;

  const name = `${account.name || ""} ${account.parent_name || ""}`.toLowerCase();
  const code = `${account.code || ""} ${account.parent_code || ""}`.toLowerCase();
  const isContra = ["cadangan", "kerugian", "penyisihan", "allowance", "provision", "contra"].some(
    (term) => name.includes(term),
  );

  return (
    !isContra &&
    (name.includes("piutang") ||
      name.includes("receivable") ||
      name.includes("intercomp") ||
      name.includes("intercompany") ||
      code.startsWith("1106-") ||
      code.includes(" 1106"))
  );
};

const ConvertConfirmedSaleToCreditModal: React.FC<
  ConvertConfirmedSaleToCreditModalProps
> = ({ isOpen, onClose, sale, onSuccess }) => {
  const { token } = useAuth();
  const toast = useToast();
  const [accounts, setAccounts] = useState<AccountCatalogItem[]>([]);
  const [loadingAccounts, setLoadingAccounts] = useState(false);
  const [saving, setSaving] = useState(false);
  const [selectedAccountId, setSelectedAccountId] = useState<number | undefined>();
  const [selectedTerms, setSelectedTerms] = useState<(typeof paymentTerms)[number]>("NET_30");
  const [dueDate, setDueDate] = useState("");

  const receivableAccounts = useMemo(
    () => accounts.filter(isReceivableAccount),
    [accounts],
  );

  useEffect(() => {
    if (!isOpen || !sale) return;

    const saleDate = toDateInputValue(sale.date);
    setSelectedTerms("NET_30");
    setSelectedAccountId(undefined);
    setDueDate(toDateInputValue(sale.due_date) || addDaysToDate(saleDate, 30));
  }, [isOpen, sale]);

  useEffect(() => {
    if (!isOpen || !token) return;

    let active = true;
    const loadAccounts = async () => {
      setLoadingAccounts(true);
      try {
        const catalog = await accountService.getAccountCatalog(token, "ASSET");
        if (active) setAccounts(catalog);
      } catch (error) {
        if (!active) return;
        console.error("Failed to load receivable accounts:", error);
        toast({
          title: "Gagal memuat akun piutang",
          description: "Silakan tutup lalu buka kembali formulir ini.",
          status: "error",
          duration: 4000,
        });
      } finally {
        if (active) setLoadingAccounts(false);
      }
    };

    void loadAccounts();
    return () => {
      active = false;
    };
  }, [isOpen, token, toast]);

  const handleSubmit = async () => {
    if (!sale || !selectedAccountId || !dueDate) {
      toast({
        title: "Data belum lengkap",
        description: "Pilih akun piutang dan isi jatuh tempo penjualan kredit.",
        status: "warning",
        duration: 4000,
      });
      return;
    }

    setSaving(true);
    try {
      await salesService.convertConfirmedSaleToCredit(sale.id, {
        payment_terms: selectedTerms,
        due_date: `${dueDate}T00:00:00.000Z`,
        receivable_account_id: selectedAccountId,
      });
      toast({
        title: "Penjualan diubah menjadi kredit",
        description:
          "Belum ada jurnal atau mutasi kas/bank yang dibuat. Invoice dan penerimaan dapat dicatat sesuai tanggal aktual.",
        status: "success",
        duration: 5000,
      });
      onSuccess();
      onClose();
    } catch (error: unknown) {
      toast({
        title: "Konversi tidak dapat dilakukan",
        description:
          getApiErrorMessage(error) ||
          "Pastikan penjualan belum memiliki invoice, jurnal, penerimaan, alokasi, atau mutasi kas/bank.",
        status: "error",
        duration: 6000,
      });
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="xl" scrollBehavior="inside">
      <ModalOverlay />
      <ModalContent>
        <ModalHeader>
          <HStack spacing={3}>
            <Box color="blue.500">
              <FiCreditCard />
            </Box>
            <Text>Ubah Menjadi Penjualan Kredit</Text>
          </HStack>
        </ModalHeader>
        <ModalCloseButton />
        <ModalBody>
          <VStack spacing={5} align="stretch">
            <Alert status="info" alignItems="flex-start" borderRadius="md">
              <AlertIcon mt={1} />
              <Text>
                Fitur ini hanya untuk penjualan berstatus Dikonfirmasi yang belum
                memiliki invoice, jurnal, penerimaan, alokasi pembayaran, atau
                mutasi kas/bank. Riwayat keuangan yang sudah ada tidak akan
                diubah.
              </Text>
            </Alert>

            <Box borderWidth="1px" borderRadius="md" p={4}>
              <Text fontWeight="semibold">{sale?.code}</Text>
              <Text fontSize="sm" color="gray.600">
                {sale?.customer?.name || "Pelanggan"} | Tanggal penjualan: {toDateInputValue(sale?.date)}
              </Text>
            </Box>

            <FormControl isRequired>
              <FormLabel>Termin Pembayaran</FormLabel>
              <Select
                value={selectedTerms}
                onChange={(event) =>
                  setSelectedTerms(event.target.value as (typeof paymentTerms)[number])
                }
              >
                {paymentTerms.map((term) => (
                  <option key={term} value={term}>
                    {term.replace("_", " ")}
                  </option>
                ))}
              </Select>
            </FormControl>

            <FormControl isRequired>
              <FormLabel>Jatuh Tempo</FormLabel>
              <Input
                type="date"
                value={dueDate}
                min={toDateInputValue(sale?.date)}
                onChange={(event) => setDueDate(event.target.value)}
              />
              <FormHelperText>
                Pembayaran pelanggan nanti dicatat dari invoice/penjualan pada tanggal uang benar-benar diterima.
              </FormHelperText>
            </FormControl>

            <FormControl isRequired>
              <FormLabel>COA Piutang</FormLabel>
              <SearchableSelect
                options={receivableAccounts}
                value={selectedAccountId}
                onChange={(value) => {
                  const accountID = Number(value);
                  setSelectedAccountId(Number.isFinite(accountID) && accountID > 0 ? accountID : undefined);
                }}
                placeholder={
                  loadingAccounts ? "Memuat akun piutang..." : "Cari kode atau nama akun piutang"
                }
                isLoading={loadingAccounts}
                isDisabled={loadingAccounts}
                displayFormat={(account) => `${account.code} - ${account.name}`}
              />
              {!loadingAccounts && receivableAccounts.length === 0 && (
                <FormHelperText color="red.500">
                  Tidak ada akun piutang detail yang aktif. Konfigurasikan COA piutang terlebih dahulu.
                </FormHelperText>
              )}
            </FormControl>
          </VStack>
        </ModalBody>
        <ModalFooter>
          <Button variant="ghost" mr={3} onClick={onClose} isDisabled={saving}>
            Batal
          </Button>
          <Button
            colorScheme="blue"
            leftIcon={saving ? <Spinner size="sm" /> : <FiCreditCard />}
            onClick={handleSubmit}
            isLoading={saving}
            loadingText="Menyimpan"
            isDisabled={loadingAccounts || receivableAccounts.length === 0}
          >
            Simpan sebagai Kredit
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export default ConvertConfirmedSaleToCreditModal;
