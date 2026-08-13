"use client";

import React, { useCallback, useEffect, useState } from "react";
import {
  Alert,
  AlertDescription,
  AlertIcon,
  Badge,
  Box,
  Button,
  Checkbox,
  Divider,
  Flex,
  FormControl,
  FormLabel,
  HStack,
  Icon,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Spinner,
  Text,
  Textarea,
  Tooltip,
  useColorModeValue,
  useDisclosure,
  useToast,
  VStack,
} from "@chakra-ui/react";
import {
  FiCheckCircle,
  FiClock,
  FiEdit3,
  FiRefreshCw,
  FiXCircle,
} from "react-icons/fi";
import { useAuth } from "@/contexts/AuthContext";
import purchaseService, {
  type PurchasePrePostingRevision,
  type PurchasePrePostingRevisionStatus,
  type PurchasePrePostingRevisionType,
} from "@/services/purchaseService";
import {
  isFinanceManagerRole,
  isFinanceStaffRole,
  isFinanceTeamRole,
  normalizeRole,
} from "@/utils/roles";

type PurchasePrePostingRevisionPanelProps = {
  purchase: {
    id: number;
    code: string;
    status: string;
    approval_status: string;
  };
  onChanged?: () => void | Promise<void>;
};

type DecisionState = {
  revision: PurchasePrePostingRevision;
  approve: boolean;
} | null;

const revisionTypeOptions: Array<{
  value: PurchasePrePostingRevisionType;
  label: string;
}> = [
  { value: "TRANSACTION_DATE", label: "Tanggal transaksi" },
  { value: "VENDOR_OR_PROJECT", label: "Vendor atau proyek" },
  { value: "ITEMS_AND_AMOUNT", label: "Item atau nominal" },
  { value: "TAX_AND_COST", label: "Pajak atau biaya" },
  { value: "PAYMENT_CONFIGURATION", label: "Metode atau rekening pembayaran" },
  { value: "OTHER", label: "Lainnya" },
];

const revisionTypeLabels = new Map(
  revisionTypeOptions.map((option) => [option.value, option.label]),
);

const statusPresentation: Record<
  PurchasePrePostingRevisionStatus,
  { label: string; colorScheme: string }
> = {
  PENDING_FINANCE_MANAGER: {
    label: "MENUNGGU FINANCE MANAGER",
    colorScheme: "orange",
  },
  OPEN_FOR_FINANCE: {
    label: "DIBUKA UNTUK REVISI FINANCE STAFF",
    colorScheme: "blue",
  },
  REJECTED: {
    label: "DITOLAK",
    colorScheme: "red",
  },
  RESUBMITTED: {
    label: "DIKIRIM ULANG UNTUK APPROVAL",
    colorScheme: "green",
  },
};

const eligibleStatuses = new Set([
  "PENDING",
  "PENDING_APPROVAL",
  "PENDING_ACCOUNTING",
  "PENDING_FINAL_APPROVAL",
]);

const actorName = (actor?: PurchasePrePostingRevision["requester"]) => {
  if (!actor) return "-";
  const name = [actor.first_name, actor.last_name]
    .filter(Boolean)
    .join(" ")
    .trim();
  return name || actor.username || actor.email || "-";
};

const formatDateTime = (value?: string | null) => {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return date.toLocaleString("id-ID", {
    dateStyle: "medium",
    timeStyle: "short",
  });
};

const parseRevisionTypes = (value: string): PurchasePrePostingRevisionType[] => {
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed)
      ? parsed.filter((type): type is PurchasePrePostingRevisionType =>
          revisionTypeLabels.has(type as PurchasePrePostingRevisionType),
        )
      : [];
  } catch {
    return [];
  }
};

const PurchasePrePostingRevisionPanel: React.FC<
  PurchasePrePostingRevisionPanelProps
> = ({ purchase, onChanged }) => {
  const { user } = useAuth();
  const toast = useToast();
  const role = normalizeRole(user?.role);
  const purchaseStatus = (purchase.status || "").toUpperCase();
  const canAccess = isFinanceTeamRole(role) || role === "admin";
  const canRequest =
    (isFinanceStaffRole(role) || role === "admin") &&
    eligibleStatuses.has(purchaseStatus);
  const canDecide = isFinanceManagerRole(role) || role === "admin";
  const requestDisclosure = useDisclosure();
  const [revisions, setRevisions] = useState<PurchasePrePostingRevision[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [selectedTypes, setSelectedTypes] = useState<
    PurchasePrePostingRevisionType[]
  >([]);
  const [reason, setReason] = useState("");
  const [decisionNotes, setDecisionNotes] = useState("");
  const [decision, setDecision] = useState<DecisionState>(null);
  const borderColor = useColorModeValue("gray.200", "whiteAlpha.300");
  const mutedText = useColorModeValue("gray.600", "gray.400");

  const loadRevisions = useCallback(async () => {
    if (!canAccess) {
      setRevisions([]);
      return;
    }

    setLoading(true);
    try {
      setRevisions(await purchaseService.listPrePostingRevisions(purchase.id));
    } catch (error: any) {
      console.error("Failed to load purchase pre-posting revisions", error);
      toast({
        title: "Riwayat revisi purchase tidak dapat dimuat",
        description:
          error?.response?.data?.error ||
          error?.message ||
          "Coba muat ulang beberapa saat lagi.",
        status: "error",
        duration: 5000,
      });
    } finally {
      setLoading(false);
    }
  }, [canAccess, purchase.id, toast]);

  useEffect(() => {
    void loadRevisions();
  }, [loadRevisions]);

  const refreshAfterChange = async () => {
    await loadRevisions();
    try {
      await onChanged?.();
    } catch (error) {
      console.error("Failed to refresh purchase after pre-posting revision", error);
      toast({
        title: "Data purchase belum dimuat ulang",
        description:
          "Aksi revisi sudah tersimpan. Silakan muat ulang detail purchase.",
        status: "warning",
        duration: 5000,
      });
    }
  };

  const toggleRevisionType = (type: PurchasePrePostingRevisionType) => {
    setSelectedTypes((current) =>
      current.includes(type)
        ? current.filter((value) => value !== type)
        : [...current, type],
    );
  };

  const requestRevision = async () => {
    if (selectedTypes.length === 0) {
      toast({
        title: "Pilih minimal satu jenis revisi",
        status: "warning",
        duration: 3500,
      });
      return;
    }
    if (!reason.trim()) {
      toast({
        title: "Alasan revisi wajib diisi",
        status: "warning",
        duration: 3500,
      });
      return;
    }

    setSaving(true);
    try {
      const revision = await purchaseService.requestPrePostingRevision(
        purchase.id,
        {
          revision_types: selectedTypes,
          reason: reason.trim(),
        },
      );
      const openedForFinance = revision.status === "OPEN_FOR_FINANCE";
      toast({
        title: openedForFinance
          ? "Purchase dibuka untuk revisi"
          : "Revisi purchase diajukan",
        description: openedForFinance
          ? "Silakan ubah data purchase lalu submit ulang melalui alur approval normal."
          : "Menunggu keputusan Finance Manager untuk membuka revisi.",
        status: "success",
        duration: 5000,
      });
      setSelectedTypes([]);
      setReason("");
      requestDisclosure.onClose();
      await refreshAfterChange();
    } catch (error: any) {
      toast({
        title: "Revisi purchase gagal diajukan",
        description:
          error?.response?.data?.error ||
          error?.message ||
          "Coba periksa status purchase lalu ulangi.",
        status: "error",
        duration: 6000,
      });
    } finally {
      setSaving(false);
    }
  };

  const openDecision = (
    revision: PurchasePrePostingRevision,
    approve: boolean,
  ) => {
    setDecisionNotes(revision.finance_manager_decision_notes || "");
    setDecision({ revision, approve });
  };

  const submitDecision = async () => {
    if (!decision) return;
    if (!decision.approve && !decisionNotes.trim()) {
      toast({
        title: "Alasan penolakan wajib diisi",
        status: "warning",
        duration: 3500,
      });
      return;
    }

    setSaving(true);
    try {
      await purchaseService.decidePrePostingRevision(
        purchase.id,
        decision.revision.id,
        {
          approve: decision.approve,
          notes: decisionNotes.trim() || undefined,
        },
      );
      toast({
        title: decision.approve
          ? "Revisi purchase disetujui"
          : "Revisi purchase ditolak",
        description: decision.approve
          ? "Purchase dikembalikan ke Finance Staff untuk diubah dan disubmit ulang."
          : "Purchase tetap berada pada tahap sebelumnya.",
        status: decision.approve ? "success" : "info",
        duration: 5000,
      });
      setDecision(null);
      await refreshAfterChange();
    } catch (error: any) {
      toast({
        title: "Keputusan revisi purchase gagal disimpan",
        description:
          error?.response?.data?.error ||
          error?.message ||
          "Coba muat ulang purchase lalu ulangi.",
        status: "error",
        duration: 6000,
      });
    } finally {
      setSaving(false);
    }
  };

  if (!canAccess || (!canRequest && revisions.length === 0 && !loading)) {
    return null;
  }

  return (
    <VStack align="stretch" spacing={4}>
      <Flex
        justify="space-between"
        align={{ base: "stretch", md: "center" }}
        direction={{ base: "column", md: "row" }}
        gap={3}
      >
        <Box>
          <HStack spacing={2}>
            <Icon as={FiEdit3} color="blue.500" />
            <Text fontWeight="bold">Revisi Data Sebelum Posting</Text>
          </HStack>
          <Text mt={1} fontSize="sm" color={mutedText}>
            Digunakan untuk mengembalikan PO ke Finance Staff sebelum jurnal
            dibuat. Tidak mengubah jurnal, kas/bank, stok, atau pembayaran.
          </Text>
        </Box>
        <HStack alignSelf={{ base: "flex-end", md: "auto" }}>
          <Tooltip label="Muat ulang riwayat revisi purchase">
            <Button
              aria-label="Muat ulang riwayat revisi purchase"
              size="sm"
              variant="outline"
              leftIcon={<FiRefreshCw />}
              onClick={() => void loadRevisions()}
              isLoading={loading}
            >
              Muat ulang
            </Button>
          </Tooltip>
          {canRequest && (
            <Button
              size="sm"
              colorScheme="blue"
              leftIcon={<FiEdit3 />}
              onClick={requestDisclosure.onOpen}
            >
              Ajukan Revisi Data
            </Button>
          )}
        </HStack>
      </Flex>

      {loading && revisions.length === 0 ? (
        <Flex py={4} justify="center">
          <Spinner />
        </Flex>
      ) : revisions.length === 0 ? (
        <Alert status="info" variant="left-accent">
          <AlertIcon />
          <AlertDescription>
            Belum ada pengajuan revisi data purchase sebelum jurnal diposting.
          </AlertDescription>
        </Alert>
      ) : (
        <VStack align="stretch" spacing={3}>
          {revisions.map((revision) => {
            const presentation = statusPresentation[revision.status] || {
              label: revision.status,
              colorScheme: "gray",
            };
            const revisionTypes = parseRevisionTypes(revision.revision_types);
            const canAct =
              canDecide &&
              revision.status === "PENDING_FINANCE_MANAGER";

            return (
              <Box
                key={revision.id}
                borderWidth="1px"
                borderColor={borderColor}
                borderRadius="md"
                p={4}
              >
                <Flex
                  justify="space-between"
                  align={{ base: "flex-start", md: "center" }}
                  direction={{ base: "column", md: "row" }}
                  gap={3}
                >
                  <Box minW={0}>
                    <HStack spacing={2} flexWrap="wrap">
                      <Text fontWeight="semibold">{purchase.code}</Text>
                      <Badge colorScheme={presentation.colorScheme}>
                        {presentation.label}
                      </Badge>
                    </HStack>
                    <Text mt={1} fontSize="sm" color={mutedText}>
                      Diajukan {formatDateTime(revision.requested_at)} oleh{" "}
                      {actorName(revision.requester)}
                    </Text>
                  </Box>
                  {canAct && (
                    <HStack flexWrap="wrap">
                      <Button
                        size="sm"
                        colorScheme="green"
                        leftIcon={<FiCheckCircle />}
                        onClick={() => openDecision(revision, true)}
                      >
                        Setujui dan Buka Revisi
                      </Button>
                      <Button
                        size="sm"
                        colorScheme="red"
                        variant="outline"
                        leftIcon={<FiXCircle />}
                        onClick={() => openDecision(revision, false)}
                      >
                        Tolak
                      </Button>
                    </HStack>
                  )}
                </Flex>

                {revisionTypes.length > 0 && (
                  <HStack mt={3} spacing={2} flexWrap="wrap">
                    {revisionTypes.map((type) => (
                      <Badge key={type} colorScheme="blue" variant="subtle">
                        {revisionTypeLabels.get(type) || type}
                      </Badge>
                    ))}
                  </HStack>
                )}

                <Text mt={3} fontSize="sm" whiteSpace="pre-wrap">
                  {revision.reason}
                </Text>

                {revision.opened_for_revision_at && (
                  <Text mt={3} fontSize="sm" color={mutedText}>
                    Dibuka untuk Finance Staff {formatDateTime(revision.opened_for_revision_at)}
                    {" oleh "}
                    {actorName(revision.opened_by)}.
                  </Text>
                )}

                {revision.resubmitted_at && (
                  <Text mt={3} fontSize="sm" color={mutedText}>
                    Disubmit ulang {formatDateTime(revision.resubmitted_at)} oleh{" "}
                    {actorName(revision.resubmitted_by_user)}.
                  </Text>
                )}

                {revision.finance_manager_decision_at && (
                  <>
                    <Divider my={3} />
                    <Text fontSize="sm" fontWeight="semibold">
                      Keputusan Finance Manager
                    </Text>
                    <Text mt={1} fontSize="sm" color={mutedText}>
                      {formatDateTime(revision.finance_manager_decision_at)} oleh{" "}
                      {actorName(revision.finance_manager)}
                    </Text>
                    {revision.finance_manager_decision_notes && (
                      <Text mt={2} fontSize="sm" whiteSpace="pre-wrap">
                        {revision.finance_manager_decision_notes}
                      </Text>
                    )}
                  </>
                )}
              </Box>
            );
          })}
        </VStack>
      )}

      <Modal
        isOpen={requestDisclosure.isOpen}
        onClose={() => !saving && requestDisclosure.onClose()}
        size="lg"
        isCentered
      >
        <ModalOverlay />
        <ModalContent
          maxH="calc(100dvh - 2rem)"
          display="flex"
          flexDirection="column"
          overflow="hidden"
        >
          <ModalHeader>Ajukan Revisi Data Purchase</ModalHeader>
          <ModalCloseButton isDisabled={saving} />
          <ModalBody flex="1" minH="0" overflowY="auto">
            <VStack align="stretch" spacing={4}>
              <Alert status="info" variant="left-accent">
                <AlertIcon />
                <AlertDescription>
                  {purchaseStatus === "PENDING" ||
                  purchaseStatus === "PENDING_APPROVAL"
                    ? "PO akan langsung dikembalikan ke Finance Staff karena approval awal belum selesai. Setelah diubah, submit ulang melalui alur approval normal."
                    : "Finance Manager akan memutuskan apakah PO dapat dikembalikan ke Finance Staff. Jurnal, kas/bank, stok, dan pembayaran tidak diubah."}
                </AlertDescription>
              </Alert>
              <FormControl isRequired>
                <FormLabel>Bagian yang perlu direvisi</FormLabel>
                <VStack align="stretch" spacing={2}>
                  {revisionTypeOptions.map((option) => (
                    <Checkbox
                      key={option.value}
                      isChecked={selectedTypes.includes(option.value)}
                      onChange={() => toggleRevisionType(option.value)}
                    >
                      {option.label}
                    </Checkbox>
                  ))}
                </VStack>
              </FormControl>
              <FormControl isRequired>
                <FormLabel>Alasan dan kronologi</FormLabel>
                <Textarea
                  value={reason}
                  onChange={(event) => setReason(event.target.value)}
                  rows={5}
                  placeholder="Jelaskan data yang perlu diperbaiki dan dasar revisinya."
                />
              </FormControl>
            </VStack>
          </ModalBody>
          <ModalFooter flexShrink={0}>
            <Button
              variant="ghost"
              mr={3}
              onClick={requestDisclosure.onClose}
              isDisabled={saving}
            >
              Batal
            </Button>
            <Button
              colorScheme="blue"
              leftIcon={<FiClock />}
              onClick={() => void requestRevision()}
              isLoading={saving}
            >
              {purchaseStatus === "PENDING" ||
              purchaseStatus === "PENDING_APPROVAL"
                ? "Buka Revisi"
                : "Kirim ke Finance Manager"}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      <Modal
        isOpen={Boolean(decision)}
        onClose={() => !saving && setDecision(null)}
        size="lg"
        isCentered
      >
        <ModalOverlay />
        <ModalContent
          maxH="calc(100dvh - 2rem)"
          display="flex"
          flexDirection="column"
          overflow="hidden"
        >
          <ModalHeader>
            {decision?.approve
              ? "Setujui Revisi Data Purchase"
              : "Tolak Revisi Data Purchase"}
          </ModalHeader>
          <ModalCloseButton isDisabled={saving} />
          <ModalBody flex="1" minH="0" overflowY="auto">
            <VStack align="stretch" spacing={4}>
              <Alert
                status={decision?.approve ? "success" : "warning"}
                variant="left-accent"
              >
                <AlertIcon />
                <AlertDescription>
                  {decision?.approve
                    ? "PO dikembalikan ke Finance Staff. COA item dikosongkan agar Accounting mengisi ulang setelah PO disubmit dan disetujui kembali."
                    : "PO tetap berada pada tahap sebelumnya dan tidak ada data akuntansi yang diubah."}
                </AlertDescription>
              </Alert>
              <FormControl isRequired={!decision?.approve}>
                <FormLabel>
                  {decision?.approve
                    ? "Catatan persetujuan (opsional)"
                    : "Alasan penolakan"}
                </FormLabel>
                <Textarea
                  value={decisionNotes}
                  onChange={(event) => setDecisionNotes(event.target.value)}
                  rows={5}
                  placeholder={
                    decision?.approve
                      ? "Tambahkan arahan revisi untuk Finance Staff bila diperlukan."
                      : "Jelaskan alasan pengajuan belum dapat disetujui."
                  }
                />
              </FormControl>
            </VStack>
          </ModalBody>
          <ModalFooter flexShrink={0}>
            <Button
              variant="ghost"
              mr={3}
              onClick={() => setDecision(null)}
              isDisabled={saving}
            >
              Batal
            </Button>
            <Button
              colorScheme={decision?.approve ? "green" : "red"}
              onClick={() => void submitDecision()}
              isLoading={saving}
            >
              {decision?.approve ? "Setujui dan Buka Revisi" : "Tolak Revisi"}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </VStack>
  );
};

export default PurchasePrePostingRevisionPanel;
