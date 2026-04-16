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
  Divider,
  Grid,
  GridItem,
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
  if (!entry) return null;

  const formatRoleLabel = (role?: string) => {
    switch (String(role || "").toLowerCase()) {
      case "finance_staff":
        return "Finance Staff";
      case "finance_manager":
      case "finance":
        return "Finance Manager";
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
      <ModalContent>
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
            <Box bg="gray.50" p={4} borderRadius="md" borderWidth="1px">
              <Grid templateColumns="repeat(2, 1fr)" gap={4}>
                <GridItem>
                  <Text fontSize="xs" color="gray.500">
                    Entry Number
                  </Text>
                  <Text fontWeight="bold">{entry.entry_number}</Text>
                </GridItem>
                <GridItem>
                  <Text fontSize="xs" color="gray.500">
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
                  <Text fontSize="xs" color="gray.500">
                    Reference
                  </Text>
                  <Text>{entry.reference || "-"}</Text>
                </GridItem>
                <GridItem>
                  <Text fontSize="xs" color="gray.500">
                    Source
                  </Text>
                  <Text>{sourceLabel}</Text>
                </GridItem>
                <GridItem colSpan={2}>
                  <Text fontSize="xs" color="gray.500">
                    Description
                  </Text>
                  <Text>{entry.description}</Text>
                </GridItem>
                {entry.adjustment_of && (
                  <GridItem colSpan={2}>
                    <Text fontSize="xs" color="gray.500">
                      Adjustment Of
                    </Text>
                    <Box
                      mt={1}
                      p={3}
                      bg="orange.50"
                      borderRadius="md"
                      borderWidth="1px"
                      borderColor="orange.100"
                    >
                      <Text fontWeight="medium">{adjustedEntryLabel}</Text>
                    </Box>
                  </GridItem>
                )}
                {entry.source_type === "ADJUSTMENT" && (
                  <GridItem colSpan={2}>
                    <Text fontSize="xs" color="gray.500">
                      Approval Flow
                    </Text>
                    <Box
                      mt={1}
                      p={3}
                      bg="orange.50"
                      borderRadius="md"
                      borderWidth="1px"
                      borderColor="orange.100"
                    >
                      <Text>
                        Staff finance membuat draft adjustment dan finance
                        manager melakukan approve/posting jurnal.
                      </Text>
                    </Box>
                  </GridItem>
                )}
                {relatedAdjustments.length > 0 && (
                  <GridItem colSpan={2}>
                    <Text fontSize="xs" color="gray.500">
                      Related Adjustments
                    </Text>
                    <VStack
                      mt={1}
                      spacing={2}
                      align="stretch"
                      bg="orange.50"
                      borderRadius="md"
                      borderWidth="1px"
                      borderColor="orange.100"
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
                            <Text fontSize="sm" color="gray.600">
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
                    <Text fontSize="xs" color="gray.500">
                      Posting Notes
                    </Text>
                    <Box
                      mt={1}
                      p={3}
                      bg="blue.50"
                      borderRadius="md"
                      borderWidth="1px"
                      borderColor="blue.100"
                    >
                      <Text fontStyle="italic" whiteSpace="pre-wrap">
                        {entry.notes}
                      </Text>
                    </Box>
                  </GridItem>
                )}
                <GridItem>
                  <Text fontSize="xs" color="gray.500">
                    Created By
                  </Text>
                  <Text>{createdByLabel}</Text>
                </GridItem>
                {entry.posted_by && (
                  <GridItem>
                    <Text fontSize="xs" color="gray.500">
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
            <Box borderWidth="1px" borderRadius="lg" overflow="hidden">
              <Table size="sm" variant="simple">
                <Thead bg="gray.50">
                  <Tr>
                    <Th>#</Th>
                    <Th>Account</Th>
                    <Th>Description</Th>
                    <Th isNumeric>Debit</Th>
                    <Th isNumeric>Credit</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {entry.lines?.map((line, index) => (
                    <Tr key={line.id}>
                      <Td>{line.line_number}</Td>
                      <Td fontWeight="medium">
                        {line.account
                          ? `${line.account.code} - ${line.account.name}`
                          : `Account #${line.account_id}`}
                      </Td>
                      <Td>{line.description}</Td>
                      <Td isNumeric>
                        {line.debit_amount > 0
                          ? formatCurrency(line.debit_amount)
                          : "-"}
                      </Td>
                      <Td isNumeric>
                        {line.credit_amount > 0
                          ? formatCurrency(line.credit_amount)
                          : "-"}
                      </Td>
                    </Tr>
                  ))}
                  {/* Totals Row */}
                  <Tr bg="gray.50" fontWeight="bold">
                    <Td colSpan={3} textAlign="right">
                      Total
                    </Td>
                    <Td isNumeric>{formatCurrency(entry.total_debit)}</Td>
                    <Td isNumeric>{formatCurrency(entry.total_credit)}</Td>
                  </Tr>
                  <Tr>
                    <Td
                      colSpan={3}
                      textAlign="right"
                      fontSize="xs"
                      color="gray.500"
                    >
                      Balance Check (Diff)
                    </Td>
                    <Td
                      colSpan={2}
                      textAlign="center"
                      fontSize="xs"
                      color={entry.is_balanced ? "green.500" : "red.500"}
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
