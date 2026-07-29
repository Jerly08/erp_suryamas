"use client";

import React, { useCallback, useEffect, useState } from "react";
import {
  Alert,
  AlertDescription,
  AlertIcon,
  Badge,
  Box,
  Button,
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
  type PurchaseCOARevision,
  type PurchaseCOARevisionStatus,
} from "@/services/purchaseService";
import {
  isAccountingRole,
  isFinanceManagerRole,
  isFinanceTeamRole,
  normalizeRole,
} from "@/utils/roles";

type PurchaseCOARevisionPanelProps = {
  purchase: {
    id: number;
    code: string;
    status: string;
    approval_status: string;
  };
  onChanged?: () => void | Promise<void>;
};

type DecisionState = {
  revision: PurchaseCOARevision;
  approve: boolean;
} | null;

const statusPresentation: Record<
  PurchaseCOARevisionStatus,
  { label: string; colorScheme: string }
> = {
  PENDING_FINANCE_MANAGER: {
    label: "MENUNGGU FINANCE MANAGER",
    colorScheme: "orange",
  },
  APPROVED: {
    label: "DISETUJUI - KEMBALI KE ACCOUNTING",
    colorScheme: "green",
  },
  REJECTED: {
    label: "DITOLAK",
    colorScheme: "red",
  },
};

const actorName = (actor?: PurchaseCOARevision["requester"]) => {
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

const PurchaseCOARevisionPanel: React.FC<PurchaseCOARevisionPanelProps> = ({
  purchase,
  onChanged,
}) => {
  const { user } = useAuth();
  const toast = useToast();
  const role = normalizeRole(user?.role);
  const canAccess = isFinanceTeamRole(role) || role === "admin";
  const canRequest =
    (isAccountingRole(role) || role === "admin") &&
    purchase.status === "PENDING_FINAL_APPROVAL" &&
    purchase.approval_status === "APPROVED";
  const canDecide = isFinanceManagerRole(role) || role === "admin";
  const requestDisclosure = useDisclosure();
  const [revisions, setRevisions] = useState<PurchaseCOARevision[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
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
      setRevisions(await purchaseService.listCOARevisions(purchase.id));
    } catch (error: any) {
      console.error("Failed to load purchase COA revisions", error);
      toast({
        title: "Riwayat revisi COA tidak dapat dimuat",
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
      console.error("Failed to refresh purchase after COA revision change", error);
      toast({
        title: "Data purchase belum dimuat ulang",
        description: "Aksi revisi COA sudah tersimpan. Silakan muat ulang detail purchase.",
        status: "warning",
        duration: 5000,
      });
    }
  };

  const requestRevision = async () => {
    if (!reason.trim()) {
      toast({
        title: "Alasan revisi COA wajib diisi",
        status: "warning",
        duration: 3500,
      });
      return;
    }

    setSaving(true);
    try {
      await purchaseService.requestCOARevision(purchase.id, {
        reason: reason.trim(),
      });
      toast({
        title: "Revisi COA diajukan",
        description: "Menunggu keputusan Finance Manager.",
        status: "success",
        duration: 4000,
      });
      setReason("");
      requestDisclosure.onClose();
      await refreshAfterChange();
    } catch (error: any) {
      toast({
        title: "Revisi COA gagal diajukan",
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

  const openDecision = (revision: PurchaseCOARevision, approve: boolean) => {
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
      await purchaseService.decideCOARevision(purchase.id, decision.revision.id, {
        approve: decision.approve,
        notes: decisionNotes.trim() || undefined,
      });
      toast({
        title: decision.approve ? "Revisi COA disetujui" : "Revisi COA ditolak",
        description: decision.approve
          ? "Purchase dikembalikan ke Accounting untuk pengisian COA ulang."
          : "Purchase tetap berada pada tahap final approval.",
        status: decision.approve ? "success" : "info",
        duration: 4500,
      });
      setDecision(null);
      await refreshAfterChange();
    } catch (error: any) {
      toast({
        title: "Keputusan revisi COA gagal disimpan",
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
            <Icon as={FiEdit3} color="purple.500" />
            <Text fontWeight="bold">Revisi COA Sebelum Posting</Text>
          </HStack>
          <Text mt={1} fontSize="sm" color={mutedText}>
            Revisi yang disetujui mengembalikan PO ke Accounting. Tidak ada COA
            atau jurnal yang diubah sampai Accounting menyimpan input ulang.
          </Text>
        </Box>
        <HStack alignSelf={{ base: "flex-end", md: "auto" }}>
          <Tooltip label="Muat ulang riwayat revisi COA">
            <Button
              aria-label="Muat ulang riwayat revisi COA"
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
              colorScheme="purple"
              leftIcon={<FiEdit3 />}
              onClick={requestDisclosure.onOpen}
            >
              Ajukan Revisi COA
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
            Belum ada pengajuan revisi COA sebelum jurnal diposting.
          </AlertDescription>
        </Alert>
      ) : (
        <VStack align="stretch" spacing={3}>
          {revisions.map((revision) => {
            const presentation =
              statusPresentation[revision.status] || {
                label: revision.status,
                colorScheme: "gray",
              };
            const canAct =
              canDecide && revision.status === "PENDING_FINANCE_MANAGER";

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
                    <HStack spacing={2} wrap="wrap">
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
                        Setujui dan Kembalikan
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

                <Text mt={3} fontSize="sm" whiteSpace="pre-wrap">
                  {revision.reason}
                </Text>

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
          maxH="calc(100vh - 32px)"
          display="flex"
          flexDirection="column"
          overflow="hidden"
        >
          <ModalHeader>Ajukan Revisi COA</ModalHeader>
          <ModalCloseButton isDisabled={saving} />
          <ModalBody flex="1" minH="0" overflowY="auto">
            <VStack align="stretch" spacing={4}>
              <Alert status="info" variant="left-accent">
                <AlertIcon />
                <AlertDescription>
                  Finance Manager akan memutuskan apakah PO {purchase.code} perlu
                  dikembalikan ke Accounting. Pengajuan ini tidak mengubah COA
                  atau jurnal.
                </AlertDescription>
              </Alert>
              <FormControl isRequired>
                <FormLabel>Alasan revisi COA</FormLabel>
                <Textarea
                  value={reason}
                  onChange={(event) => setReason(event.target.value)}
                  rows={5}
                  placeholder="Jelaskan COA yang perlu diperbaiki dan dasar koreksinya."
                />
              </FormControl>
            </VStack>
          </ModalBody>
          <ModalFooter>
            <Button
              variant="ghost"
              mr={3}
              onClick={requestDisclosure.onClose}
              isDisabled={saving}
            >
              Batal
            </Button>
            <Button
              colorScheme="purple"
              leftIcon={<FiClock />}
              onClick={() => void requestRevision()}
              isLoading={saving}
            >
              Kirim ke Finance Manager
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
          maxH="calc(100vh - 32px)"
          display="flex"
          flexDirection="column"
          overflow="hidden"
        >
          <ModalHeader>
            {decision?.approve
              ? "Setujui Revisi COA"
              : "Tolak Revisi COA"}
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
                    ? "PO akan kembali ke Accounting untuk pengisian COA ulang. Jurnal belum dibuat."
                    : "PO tetap menunggu final approval dan COA saat ini tidak diubah."}
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
                      ? "Tambahkan arahan untuk Accounting bila diperlukan."
                      : "Jelaskan alasan pengajuan belum dapat disetujui."
                  }
                />
              </FormControl>
            </VStack>
          </ModalBody>
          <ModalFooter>
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
              {decision?.approve ? "Setujui dan Kembalikan" : "Tolak Revisi"}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </VStack>
  );
};

export default PurchaseCOARevisionPanel;
