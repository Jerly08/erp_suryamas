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
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    setDate(new Date().toISOString().split("T")[0]);
    setReference("");
    setNotes("");
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

      const payload: SalePPh23SettlementRequest = {
        date: new Date(date).toISOString(),
        reference: reference.trim(),
        notes: notes.trim(),
      };

      const updatedSale = await salesService.settleSalePPh23(sale.id, payload);
      const effectiveDate =
        updatedSale.pph23_settlement_date &&
        salesService.formatDate(updatedSale.pph23_settlement_date);

      toast({
        title: "PPh23 berhasil dipost",
        description: effectiveDate
          ? `Jurnal reclass PPh23 berhasil dipost pada ${effectiveDate}.`
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
        title: "Gagal mempost PPh23",
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
        <ModalHeader>Posting PPh23</ModalHeader>
        <ModalCloseButton />

        <form onSubmit={handleSubmit}>
          <ModalBody>
            {sale && (
              <VStack spacing={4} align="stretch">
                <Text fontSize="sm" color="gray.600">
                  Gunakan form ini untuk mempost jurnal susulan PPh23 setelah
                  invoice lunas dan bukti potong sudah diterima dari konsultan
                  pajak.
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
                      {sale.invoice_number || sale.code}
                    </Text>
                    <Text fontSize="sm">
                      Customer: {sale.customer?.name || "-"}
                    </Text>
                    <Text fontSize="sm">
                      Tanggal Invoice: {salesService.formatDate(sale.date)}
                    </Text>
                    <Text fontSize="sm">
                      Nilai PPh23:{" "}
                      {salesService.formatCurrency(sale.pph23_amount || 0)}
                    </Text>
                    <Text fontSize="sm" color="orange.500">
                      Jurnal reclass yang dibuat: Dr Kredit Pajak PPh23, Cr
                      PPh23 Dibayar Dimuka
                    </Text>
                  </VStack>
                </Box>

                <FormControl isRequired>
                  <FormLabel>Tanggal Posting PPh23</FormLabel>
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
              Posting PPh23
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  );
};

export default PPh23SettlementForm;
