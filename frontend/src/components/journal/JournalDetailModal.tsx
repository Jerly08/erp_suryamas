import React from "react";
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  Button,
  VStack,
  HStack,
  Text,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  Badge,
  Box,
  Grid,
  GridItem,
  useColorModeValue,
} from "@chakra-ui/react";
import { SSOTJournalEntry } from "@/services/ssotJournalService";
import { formatAccountingDate } from "@/utils/accountingDate";
import { getUserDisplayName } from "@/utils/userDisplay";

interface JournalDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  entry?: SSOTJournalEntry;
}

export default function JournalDetailModal({
  isOpen,
  onClose,
  entry,
}: JournalDetailModalProps) {
  const modalBg = useColorModeValue("white", "gray.800");
  const panelBg = useColorModeValue("gray.50", "whiteAlpha.50");
  const tableHeaderBg = useColorModeValue("gray.50", "whiteAlpha.100");
  const borderColor = useColorModeValue("gray.200", "whiteAlpha.200");
  const textColor = useColorModeValue("gray.800", "gray.100");
  const secondaryTextColor = useColorModeValue("gray.700", "gray.200");
  const mutedTextColor = useColorModeValue("gray.500", "gray.400");
  const orangePanelBg = useColorModeValue("orange.50", "orange.900");
  const orangePanelBorder = useColorModeValue("orange.100", "orange.700");
  const greenPanelBg = useColorModeValue("green.50", "green.900");
  const greenPanelBorder = useColorModeValue("green.100", "green.700");
  const bluePanelBg = useColorModeValue("blue.50", "blue.900");
  const bluePanelBorder = useColorModeValue("blue.100", "blue.700");
  const balancedColor = useColorModeValue("green.500", "green.300");
  const unbalancedColor = useColorModeValue("red.500", "red.300");

  if (!entry) return null;

  const formatRoleLabel = (role?: string) => {
    switch (String(role || "").toLowerCase()) {
      case "finance_staff":
        return "Finance Staff";
      case "accounting":
        return "Accounting";
      case "finance_manager":
      case "finance":
        return "Finance Manager";
      case "director":
        return "Director";
      case "admin":
        return "Admin";
      default:
        return "";
    }
  };

  const sourceLabel =
    entry.source_type === "ADJUSTMENT"
      ? "Jurnal Adjustment"
      : entry.source_type || "MANUAL";
  const adjustedEntryLabel = entry.adjusted_entry
    ? `${entry.adjusted_entry.entry_number}${entry.adjusted_entry.reference ? ` • ${entry.adjusted_entry.reference}` : ""}`
    : entry.adjustment_of
      ? `Journal #${entry.adjustment_of}`
      : "";
  const relatedAdjustments = entry.adjustment_entries || [];

  const createdByLabel = entry.creator
    ? `${getUserDisplayName(entry.creator, {
        fallback: `User #${entry.created_by}`,
        allowRoleFallback: false,
      })}${formatRoleLabel(entry.creator.role) ? ` (${formatRoleLabel(entry.creator.role)})` : ""}`
    : `User #${entry.created_by}`;

  const postedByLabel =
    entry.posted_by == null
      ? "-"
      : entry.poster
        ? `${getUserDisplayName(entry.poster, {
            fallback: `User #${entry.posted_by}`,
            allowRoleFallback: false,
          })}${formatRoleLabel(entry.poster.role) ? ` (${formatRoleLabel(entry.poster.role)})` : ""}`
        : `User #${entry.posted_by}`;

  const financeManagerApprovedLabel =
    entry.finance_manager_approved_by == null
      ? "-"
      : entry.finance_manager_approver
        ? `${getUserDisplayName(entry.finance_manager_approver, {
            fallback: `User #${entry.finance_manager_approved_by}`,
            allowRoleFallback: false,
          })}${formatRoleLabel(entry.finance_manager_approver.role) ? ` (${formatRoleLabel(entry.finance_manager_approver.role)})` : ""}`
        : `User #${entry.finance_manager_approved_by}`;

  const approvalStageLabel =
    entry.status !== "DRAFT"
      ? "-"
      : String(entry.approval_stage || "").toUpperCase() === "DIRECTOR"
        ? "Menunggu Director"
        : "Menunggu Finance Manager";
  const adjustmentPanelBg = entry.adjustment_completed
    ? greenPanelBg
    : orangePanelBg;
  const adjustmentPanelBorder = entry.adjustment_completed
    ? greenPanelBorder
    : orangePanelBorder;

  const formatCurrency = (val: number) => {
    return new Intl.NumberFormat("id-ID", {
      style: "currency",
      currency: "IDR",
      minimumFractionDigits: 0,
    }).format(val);
  };

  const formatDateTime = (dateStr: string) => {
    if (!dateStr) return "-";
    return new Date(dateStr).toLocaleString("id-ID", {
      weekday: "long",
      year: "numeric",
      month: "long",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="4xl" scrollBehavior="inside">
      <ModalOverlay />
      <ModalContent bg={modalBg} color={textColor}>
        <ModalHeader>
          <HStack>
            <Text>Journal Entry</Text>
            <Badge
              colorScheme={
                entry.status === "POSTED"
                  ? "green"
                  : entry.status === "DRAFT"
                    ? "yellow"
                    : "red"
              }
              fontSize="sm"
            >
              {entry.status}
            </Badge>
          </HStack>
        </ModalHeader>
        <ModalCloseButton />
        <ModalBody>
          <VStack spacing={6} align="stretch">
            {/* Header Info */}
            <Box
              bg={panelBg}
              p={4}
              borderRadius="md"
              borderWidth="1px"
              borderColor={borderColor}
            >
              <Grid templateColumns="repeat(2, 1fr)" gap={4}>
                <GridItem>
                  <Text fontSize="xs" color={mutedTextColor}>
                    Entry Number
                  </Text>
                  <Text fontWeight="bold">{entry.entry_number}</Text>
                </GridItem>
                <GridItem>
                  <Text fontSize="xs" color={mutedTextColor}>
                    Date
                  </Text>
                  <Text fontWeight="bold">
                    {formatAccountingDate(entry.entry_date, "id-ID", {
                      weekday: "long",
                      year: "numeric",
                      month: "long",
                      day: "numeric",
                    })}
                  </Text>
                </GridItem>
                <GridItem>
                  <Text fontSize="xs" color={mutedTextColor}>
                    Reference
                  </Text>
                  <Text>{entry.reference || "-"}</Text>
                </GridItem>
                <GridItem>
                  <Text fontSize="xs" color={mutedTextColor}>
                    Source
                  </Text>
                  <Text>{sourceLabel}</Text>
                </GridItem>
                <GridItem colSpan={2}>
                  <Text fontSize="xs" color={mutedTextColor}>
                    Description
                  </Text>
                  <Text>{entry.description}</Text>
                </GridItem>
                {entry.adjustment_required && (
                  <GridItem colSpan={2}>
                    <Text fontSize="xs" color={mutedTextColor}>
                      Adjustment Requirement
                    </Text>
                    <Box
                      mt={1}
                      p={3}
                      bg={adjustmentPanelBg}
                      borderRadius="md"
                      borderWidth="1px"
                      borderColor={adjustmentPanelBorder}
                    >
                      <HStack align="start" justify="space-between">
                        <Box>
                          <Text fontWeight="medium">
                            {entry.adjustment_requirement_label}
                          </Text>
                          <Text fontSize="sm" color={secondaryTextColor} mt={1}>
                            {entry.adjustment_requirement_tagline ||
                              "Jurnal ini masuk mapping khusus adjustment client."}
                          </Text>
                        </Box>
                        <Badge
                          colorScheme={
                            entry.adjustment_completed ? "green" : "orange"
                          }
                        >
                          {entry.adjustment_completed
                            ? "Sudah Adjustment"
                            : "Butuh Adjustment"}
                        </Badge>
                      </HStack>
                      {entry.adjustment_requirement_coa && (
                        <Text fontSize="sm" color={secondaryTextColor} mt={2}>
                          COA: {entry.adjustment_requirement_coa}
                        </Text>
                      )}
                      {entry.adjustment_requirement_reason && (
                        <Text fontSize="sm" color={mutedTextColor} mt={1}>
                          Alasan: {entry.adjustment_requirement_reason}
                        </Text>
                      )}
                    </Box>
                  </GridItem>
                )}
                {entry.adjustment_of && (
                  <GridItem colSpan={2}>
                    <Text fontSize="xs" color={mutedTextColor}>
                      Adjustment Of
                    </Text>
                    <Box
                      mt={1}
                      p={3}
                      bg={orangePanelBg}
                      borderRadius="md"
                      borderWidth="1px"
                      borderColor={orangePanelBorder}
                    >
                      <Text fontWeight="medium">{adjustedEntryLabel}</Text>
                    </Box>
                  </GridItem>
                )}
                {entry.source_type === "ADJUSTMENT" &&
                  (entry.adjustment_category_label ||
                    entry.adjustment_category_code) && (
                    <GridItem colSpan={2}>
                      <Text fontSize="xs" color={mutedTextColor}>
                        Adjustment Category
                      </Text>
                      <Box
                        mt={1}
                        p={3}
                        bg={orangePanelBg}
                        borderRadius="md"
                        borderWidth="1px"
                        borderColor={orangePanelBorder}
                      >
                        <Text fontWeight="medium">
                          {entry.adjustment_category_label ||
                            entry.adjustment_category_code}
                        </Text>
                        {entry.adjustment_category_code && (
                          <Text fontSize="xs" color={mutedTextColor} mt={1}>
                            {entry.adjustment_category_code}
                          </Text>
                        )}
                      </Box>
                    </GridItem>
                  )}
                {entry.source_type === "ADJUSTMENT" && (
                  <GridItem colSpan={2}>
                    <Text fontSize="xs" color={mutedTextColor}>
                      Approval Flow
                    </Text>
                    <Box
                      mt={1}
                      p={3}
                      bg={orangePanelBg}
                      borderRadius="md"
                      borderWidth="1px"
                      borderColor={orangePanelBorder}
                    >
                      <Text>
                        Finance staff atau finance manager dapat membuat draft
                        adjustment. Finance manager melakukan review awal, dan
                        jika diminta maka director akan melakukan approval final
                        sebelum jurnal diposting.
                      </Text>
                      <Text mt={2} fontSize="sm">
                        Tahap saat ini: {approvalStageLabel}
                      </Text>
                      {entry.finance_manager_approved_at && (
                        <Text mt={1} fontSize="sm">
                          Finance Manager: {financeManagerApprovedLabel} pada{" "}
                          {formatDateTime(entry.finance_manager_approved_at)}
                        </Text>
                      )}
                    </Box>
                  </GridItem>
                )}
                {relatedAdjustments.length > 0 && (
                  <GridItem colSpan={2}>
                    <Text fontSize="xs" color={mutedTextColor}>
                      Related Adjustments
                    </Text>
                    <VStack
                      mt={1}
                      spacing={2}
                      align="stretch"
                      bg={orangePanelBg}
                      borderRadius="md"
                      borderWidth="1px"
                      borderColor={orangePanelBorder}
                      p={3}
                    >
                      {relatedAdjustments.map((adjustment) => (
                        <HStack
                          key={adjustment.id}
                          justify="space-between"
                          align="start"
                        >
                          <Box>
                            <Text fontWeight="medium">
                              {adjustment.entry_number}
                            </Text>
                            <Text fontSize="sm" color={mutedTextColor}>
                              {adjustment.description}
                            </Text>
                          </Box>
                          <Badge
                            colorScheme={
                              adjustment.status === "POSTED"
                                ? "green"
                                : adjustment.status === "DRAFT"
                                  ? "yellow"
                                  : "red"
                            }
                          >
                            {adjustment.status}
                          </Badge>
                        </HStack>
                      ))}
                    </VStack>
                  </GridItem>
                )}
                {entry.notes && (
                  <GridItem colSpan={2}>
                    <Text fontSize="xs" color={mutedTextColor}>
                      Posting Notes
                    </Text>
                    <Box
                      mt={1}
                      p={3}
                      bg={bluePanelBg}
                      borderRadius="md"
                      borderWidth="1px"
                      borderColor={bluePanelBorder}
                    >
                      <Text fontStyle="italic" whiteSpace="pre-wrap">
                        {entry.notes}
                      </Text>
                    </Box>
                  </GridItem>
                )}
                <GridItem>
                  <Text fontSize="xs" color={mutedTextColor}>
                    Created By
                  </Text>
                  <Text>{createdByLabel}</Text>
                </GridItem>
                {entry.posted_by && (
                  <GridItem>
                    <Text fontSize="xs" color={mutedTextColor}>
                      {entry.source_type === "ADJUSTMENT"
                        ? "Approved / Posted By"
                        : "Posted By"}
                    </Text>
                    <Text>
                      {postedByLabel} at {formatDateTime(entry.posted_at || "")}
                    </Text>
                  </GridItem>
                )}
              </Grid>
            </Box>

            {/* Lines Table */}
            <Box
              borderWidth="1px"
              borderRadius="lg"
              borderColor={borderColor}
              overflow="hidden"
            >
              <Table size="sm" variant="simple" color={textColor}>
                <Thead bg={tableHeaderBg}>
                  <Tr>
                    <Th color={mutedTextColor} borderColor={borderColor}>#</Th>
                    <Th color={mutedTextColor} borderColor={borderColor}>
                      Account
                    </Th>
                    <Th color={mutedTextColor} borderColor={borderColor}>
                      Description
                    </Th>
                    <Th isNumeric color={mutedTextColor} borderColor={borderColor}>
                      Debit
                    </Th>
                    <Th isNumeric color={mutedTextColor} borderColor={borderColor}>
                      Credit
                    </Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {entry.lines?.map((line) => (
                    <Tr key={line.id}>
                      <Td borderColor={borderColor}>{line.line_number}</Td>
                      <Td fontWeight="medium" borderColor={borderColor}>
                        {line.account
                          ? `${line.account.code} - ${line.account.name}`
                          : `Account #${line.account_id}`}
                      </Td>
                      <Td borderColor={borderColor}>{line.description}</Td>
                      <Td isNumeric borderColor={borderColor}>
                        {line.debit_amount > 0
                          ? formatCurrency(line.debit_amount)
                          : "-"}
                      </Td>
                      <Td isNumeric borderColor={borderColor}>
                        {line.credit_amount > 0
                          ? formatCurrency(line.credit_amount)
                          : "-"}
                      </Td>
                    </Tr>
                  ))}
                  {/* Totals Row */}
                  <Tr bg={tableHeaderBg} fontWeight="bold">
                    <Td colSpan={3} textAlign="right" borderColor={borderColor}>
                      Total
                    </Td>
                    <Td isNumeric borderColor={borderColor}>
                      {formatCurrency(entry.total_debit)}
                    </Td>
                    <Td isNumeric borderColor={borderColor}>
                      {formatCurrency(entry.total_credit)}
                    </Td>
                  </Tr>
                  <Tr>
                    <Td
                      colSpan={3}
                      textAlign="right"
                      fontSize="xs"
                      color={mutedTextColor}
                      borderColor={borderColor}
                    >
                      Balance Check (Diff)
                    </Td>
                    <Td
                      colSpan={2}
                      textAlign="center"
                      fontSize="xs"
                      color={entry.is_balanced ? balancedColor : unbalancedColor}
                      borderColor={borderColor}
                    >
                      {formatCurrency(
                        Math.abs(entry.total_debit - entry.total_credit),
                      )}
                    </Td>
                  </Tr>
                </Tbody>
              </Table>
            </Box>
          </VStack>
        </ModalBody>
        <ModalFooter>
          <Button onClick={onClose}>Close</Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
}
