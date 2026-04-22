"use client";

import React, { useEffect, useState } from "react";
import {
  Box,
  Button,
  FormControl,
  FormHelperText,
  FormLabel,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Text,
  Textarea,
  VStack,
  useColorModeValue,
  useToast,
} from "@chakra-ui/react";
import salesService, {
  Sale,
  SaleLatePPh23Request,
  SalePPh23SettlementRequest,
} from "@/services/salesService";

interface PPh23SettlementFormProps {
  isOpen: boolean;
  onClose: () => void;
  sale: Sale | null;
  onSave: () => void;
}

const PPh23SettlementForm: React.FC<PPh23SettlementFormProps> = ({
  isOpen,
  onClose,
  sale,
  onSave,
}) => {
  const toast = useToast();
  const panelBg = useColorModeValue("gray.50", "whiteAlpha.100");
  const panelBorderColor = useColorModeValue("gray.200", "whiteAlpha.200");
  const [date, setDate] = useState("");
  const [reference, setReference] = useState("");
  const [notes, setNotes] = useState("");
  const [amount, setAmount] = useState("");
  const [rate, setRate] = useState("");
  const [loading, setLoading] = useState(false);
  const isLateAddMode =
    !!sale &&
    (sale.pph23_amount || 0) <= 0 &&
    !sale.pph23_settlement_date &&
    sale.status !== "CANCELLED";
  const isInvoiceSettled = (sale?.outstanding_amount || 0) <= 0.005;
  const referenceNumber = sale?.invoice_number || sale?.code || "-";

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    setDate(new Date().toISOString().split("T")[0]);
    setReference("");
    setNotes("");
    setAmount("");
    setRate("");
  }, [isOpen, sale]);

  const handleClose = () => {
    if (loading) {
      return;
    }
    onClose();
  };

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!sale) {
      return;
    }

    if (!date) {
      toast({
        title: "Tanggal wajib diisi",
        status: "error",
        duration: 3000,
      });
      return;
    }

    try {
      setLoading(true);

      let updatedSale: Sale;
      if (isLateAddMode) {
        const numericAmount = Number(amount);
        const numericRate = Number(rate);

        if ((!amount || Number.isNaN(numericAmount) || numericAmount <= 0) &&
            (!rate || Number.isNaN(numericRate) || numericRate <= 0)) {
          toast({
            title: "Nominal atau tarif wajib diisi",
            description: "Isi nominal PPh23 atau tarif PPh23 agar sistem bisa memproses jurnal susulan.",
            status: "error",
            duration: 4000,
          });
          return;
        }

        const payload: SaleLatePPh23Request = {
          date: new Date(date).toISOString(),
          reference: reference.trim(),
          notes: notes.trim(),
        };
        if (!Number.isNaN(numericAmount) && numericAmount > 0) {
          payload.amount = numericAmount;
        }
        if (!Number.isNaN(numericRate) && numericRate > 0) {
          payload.rate = numericRate;
        }

        updatedSale = await salesService.addLateSalePPh23(sale.id, payload);
      } else {
        const payload: SalePPh23SettlementRequest = {
          date: new Date(date).toISOString(),
          reference: reference.trim(),
          notes: notes.trim(),
        };

        updatedSale = await salesService.settleSalePPh23(sale.id, payload);
      }
      const effectiveDate =
        updatedSale.pph23_settlement_date &&
        salesService.formatDate(updatedSale.pph23_settlement_date);

      toast({
        title: isLateAddMode
          ? "PPh23 susulan berhasil ditambahkan"
          : "PPh23 berhasil dipost",
        description: effectiveDate
          ? isLateAddMode
            ? `PPh23 susulan berhasil dipost pada ${effectiveDate}.`
            : `Jurnal reclass PPh23 berhasil dipost pada ${effectiveDate}.`
          : isLateAddMode
            ? "PPh23 susulan berhasil dipost."
            : "Jurnal reclass PPh23 berhasil dipost.",
        status: "success",
        duration: 4000,
      });

      onSave();
      onClose();
    } catch (error: any) {
      const errorMessage =
        error.response?.data?.context?.details ||
        error.response?.data?.details ||
        error.response?.data?.message ||
        "Terjadi kesalahan saat memproses PPh23.";

      toast({
        title: isLateAddMode
          ? "Gagal menambahkan PPh23"
          : "Gagal mempost PPh23",
        description: errorMessage,
        status: "error",
        duration: 5000,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={handleClose} size="lg" isCentered>
      <ModalOverlay />
      <ModalContent>
        <ModalHeader>
          {isLateAddMode ? "Tambah PPh23 Susulan" : "Posting PPh23"}
        </ModalHeader>
        <ModalCloseButton />

        <form onSubmit={handleSubmit}>
          <ModalBody>
            {sale && (
              <VStack spacing={4} align="stretch">
                <Text fontSize="sm" color="gray.600">
                  {isLateAddMode
                    ? "Gunakan form ini saat invoice awalnya belum diisi PPh23, lalu bukti potong / info dari konsultan pajak baru diterima belakangan."
                    : "Gunakan form ini untuk mempost jurnal susulan PPh23 setelah invoice lunas dan bukti potong sudah diterima dari konsultan pajak."}
                </Text>

                <Box
                  p={4}
                  borderWidth="1px"
                  borderColor={panelBorderColor}
                  borderRadius="md"
                  bg={panelBg}
                >
                  <VStack align="stretch" spacing={2}>
                    <Text fontWeight="semibold">
                      {referenceNumber}
                    </Text>
                    <Text fontSize="sm">
                      Customer: {sale.customer?.name || "-"}
                    </Text>
                    <Text fontSize="sm">
                      Tanggal Invoice: {salesService.formatDate(sale.date)}
                    </Text>
                    <Text fontSize="sm">
                      DPP Pajak:{" "}
                      {salesService.formatCurrency(sale.taxable_amount || 0)}
                    </Text>
                    <Text fontSize="sm">
                      Nilai Invoice Saat Ini:{" "}
                      {salesService.formatCurrency(sale.total_amount || 0)}
                    </Text>
                    {isLateAddMode ? (
                      <>
                        <Text fontSize="sm">
                          Outstanding Saat Ini:{" "}
                          {salesService.formatCurrency(
                            sale.outstanding_amount || 0,
                          )}
                        </Text>
                        <Text fontSize="sm" color="orange.500">
                          {isInvoiceSettled
                            ? "Invoice sudah lunas. Sistem akan koreksi payment/kas-bank dulu, lalu mempost PPh23 final pada tanggal yang dipilih."
                            : "Invoice belum sepenuhnya lunas. Sistem akan mempost PPh23 final jika nilai PPh23 yang diinput menutup sisa piutang invoice."}
                        </Text>
                      </>
                    ) : (
                      <>
                        <Text fontSize="sm">
                          Nilai PPh23:{" "}
                          {salesService.formatCurrency(sale.pph23_amount || 0)}
                        </Text>
                        <Text fontSize="sm" color="orange.500">
                          Jurnal reclass yang dibuat: Dr Kredit Pajak PPh23, Cr
                          PPh23 Dibayar Dimuka
                        </Text>
                      </>
                    )}
                  </VStack>
                </Box>

                <FormControl isRequired>
                  <FormLabel>
                    {isLateAddMode
                      ? "Tanggal Bukti Potong / Posting PPh23"
                      : "Tanggal Posting PPh23"}
                  </FormLabel>
                  <Input
                    type="date"
                    value={date}
                    onChange={(event) => setDate(event.target.value)}
                  />
                  <FormHelperText>
                    Tanggal ini boleh berbeda dari tanggal invoice karena
                    mengikuti info konsultan pajak.
                  </FormHelperText>
                </FormControl>

                {isLateAddMode && (
                  <>
                    <FormControl>
                      <FormLabel>Nominal PPh23</FormLabel>
                      <Input
                        type="number"
                        min={0}
                        step="0.01"
                        value={amount}
                        onChange={(event) => setAmount(event.target.value)}
                        placeholder="Contoh: 2000000"
                      />
                      <FormHelperText>
                        Isi nominal jika nilai PPh23 sudah diketahui dari
                        bukti potong atau konsultan pajak.
                      </FormHelperText>
                    </FormControl>

                    <FormControl>
                      <FormLabel>Tarif PPh23</FormLabel>
                      <Input
                        type="number"
                        min={0}
                        step="0.01"
                        value={rate}
                        onChange={(event) => setRate(event.target.value)}
                        placeholder="Contoh: 2"
                      />
                      <FormHelperText>
                        Opsional. Jika nominal belum diisi, sistem akan hitung
                        otomatis dari DPP pajak invoice ini.
                      </FormHelperText>
                    </FormControl>
                  </>
                )}

                <FormControl>
                  <FormLabel>Referensi / Nomor Bukti Potong</FormLabel>
                  <Input
                    value={reference}
                    onChange={(event) => setReference(event.target.value)}
                    placeholder="Contoh: BUPOT-001"
                  />
                </FormControl>

                <FormControl>
                  <FormLabel>Catatan</FormLabel>
                  <Textarea
                    value={notes}
                    onChange={(event) => setNotes(event.target.value)}
                    placeholder="Opsional, misalnya info dari konsultan pajak"
                    rows={4}
                  />
                </FormControl>
              </VStack>
            )}
          </ModalBody>

          <ModalFooter>
            <Button variant="ghost" mr={3} onClick={handleClose}>
              Batal
            </Button>
            <Button colorScheme="blue" type="submit" isLoading={loading}>
              {isLateAddMode ? "Tambah & Posting PPh23" : "Posting PPh23"}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  );
};

export default PPh23SettlementForm;
