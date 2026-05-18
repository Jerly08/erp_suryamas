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
  const currentPPh23Amount = sale?.pph23_amount || 0;
  const hasExistingPPh23 = currentPPh23Amount > 0.005;
  const isLateAddMode =
    !!sale &&
    !hasExistingPPh23 &&
    !sale.pph23_settlement_date &&
    sale.status !== "CANCELLED";
  const canAdjustExistingPPh23 =
    !!sale &&
    hasExistingPPh23 &&
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
      const numericAmount = Number(amount);
      const numericRate = Number(rate);
      const hasAmountInput =
        amount.trim() !== "" && !Number.isNaN(numericAmount) && numericAmount > 0;
      const hasRateInput =
        rate.trim() !== "" && !Number.isNaN(numericRate) && numericRate > 0;
      const shouldPostLateOrAdjustment =
        isLateAddMode || (canAdjustExistingPPh23 && (hasAmountInput || hasRateInput));

      if (shouldPostLateOrAdjustment) {
        if (isLateAddMode && !hasAmountInput && !hasRateInput) {
          toast({
            title: "Nominal atau tarif wajib diisi",
            description: "Isi nominal PPh23 atau tarif PPh23 agar sistem bisa memproses jurnal susulan.",
            status: "error",
            duration: 4000,
          });
          return;
        }
        if (
          canAdjustExistingPPh23 &&
          hasAmountInput &&
          numericAmount <= currentPPh23Amount
        ) {
          toast({
            title: "Nominal final belum lebih besar",
            description: `Nominal PPh23 final harus lebih besar dari PPh23 yang sudah tercatat (${salesService.formatCurrency(currentPPh23Amount)}).`,
            status: "error",
            duration: 5000,
          });
          return;
        }

        const payload: SaleLatePPh23Request = {
          date: new Date(date).toISOString(),
          reference: reference.trim(),
          notes: notes.trim(),
        };
        if (hasAmountInput) {
          payload.amount = numericAmount;
        }
        if (hasRateInput) {
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
          : shouldPostLateOrAdjustment
            ? "Koreksi PPh23 berhasil dipost"
            : "PPh23 berhasil dipost",
        description: effectiveDate
          ? isLateAddMode
            ? `PPh23 susulan berhasil dipost pada ${effectiveDate}.`
            : shouldPostLateOrAdjustment
              ? `Koreksi kurang bayar PPh23 berhasil dipost pada ${effectiveDate}.`
              : `Jurnal reclass PPh23 berhasil dipost pada ${effectiveDate}.`
          : isLateAddMode
            ? "PPh23 susulan berhasil dipost."
            : shouldPostLateOrAdjustment
              ? "Koreksi kurang bayar PPh23 berhasil dipost."
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
          : amount.trim() !== "" || rate.trim() !== ""
            ? "Gagal koreksi PPh23"
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
          {isLateAddMode
            ? "Tambah PPh23 Susulan"
            : canAdjustExistingPPh23
              ? "Posting / Koreksi PPh23"
              : "Posting PPh23"}
        </ModalHeader>
        <ModalCloseButton />

        <form onSubmit={handleSubmit}>
          <ModalBody>
            {sale && (
              <VStack spacing={4} align="stretch">
                <Text fontSize="sm" color="gray.600">
                  {isLateAddMode
                    ? "Gunakan form ini saat invoice awalnya belum diisi PPh23, lalu bukti potong / info dari konsultan pajak baru diterima belakangan."
                    : canAdjustExistingPPh23
                      ? "Gunakan form ini untuk mempost PPh23 saat bukti potong diterima. Jika ada kurang bayar, isi nominal atau tarif final yang lebih besar dari PPh23 saat ini."
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
                    {(isLateAddMode || canAdjustExistingPPh23) ? (
                      <>
                        {canAdjustExistingPPh23 && (
                          <Text fontSize="sm">
                            PPh23 Tercatat Saat Ini:{" "}
                            {salesService.formatCurrency(currentPPh23Amount)}
                          </Text>
                        )}
                        <Text fontSize="sm">
                          Outstanding Saat Ini:{" "}
                          {salesService.formatCurrency(
                            sale.outstanding_amount || 0,
                          )}
                        </Text>
                        <Text fontSize="sm" color="orange.500">
                          {isInvoiceSettled
                            ? "Invoice sudah lunas. Jika ada tambahan PPh23, sistem akan koreksi payment/kas-bank dulu, lalu mempost PPh23 final pada tanggal yang dipilih."
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

                {(isLateAddMode || canAdjustExistingPPh23) && (
                  <>
                    <FormControl>
                      <FormLabel>
                        {canAdjustExistingPPh23
                          ? "Nominal PPh23 Final"
                          : "Nominal PPh23"}
                      </FormLabel>
                      <Input
                        type="number"
                        min={0}
                        step="0.01"
                        value={amount}
                        onChange={(event) => setAmount(event.target.value)}
                        placeholder="Contoh: 2000000"
                      />
                      <FormHelperText>
                        {canAdjustExistingPPh23
                          ? "Opsional. Isi hanya jika bukti potong menunjukkan PPh23 final lebih besar dari nilai yang tercatat."
                          : "Isi nominal jika nilai PPh23 sudah diketahui dari bukti potong atau konsultan pajak."}
                      </FormHelperText>
                    </FormControl>

                    <FormControl>
                      <FormLabel>
                        {canAdjustExistingPPh23
                          ? "Tarif PPh23 Final"
                          : "Tarif PPh23"}
                      </FormLabel>
                      <Input
                        type="number"
                        min={0}
                        step="0.01"
                        value={rate}
                        onChange={(event) => setRate(event.target.value)}
                        placeholder="Contoh: 2"
                      />
                      <FormHelperText>
                        {canAdjustExistingPPh23
                          ? "Opsional. Jika nominal final belum diisi, sistem menghitung nominal final dari DPP pajak invoice ini."
                          : "Opsional. Jika nominal belum diisi, sistem akan hitung otomatis dari DPP pajak invoice ini."}
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
              {isLateAddMode
                ? "Tambah & Posting PPh23"
                : canAdjustExistingPPh23
                  ? "Posting / Koreksi PPh23"
                  : "Posting PPh23"}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  );
};

export default PPh23SettlementForm;
