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

  const createdByLabel = entry.creator
    ? getUserDisplayName(entry.creator, {
        fallback: `User #${entry.created_by}`,
        allowRoleFallback: false,
      })
    : `User #${entry.created_by}`;

  const postedByLabel =
    entry.posted_by == null
      ? "-"
      : entry.poster
        ? getUserDisplayName(entry.poster, {
            fallback: `User #${entry.posted_by}`,
            allowRoleFallback: false,
          })
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
                  <Text>{entry.source_type || "MANUAL"}</Text>
                </GridItem>
                <GridItem colSpan={2}>
                  <Text fontSize="xs" color="gray.500">
                    Description
                  </Text>
                  <Text>{entry.description}</Text>
                </GridItem>
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
                      Posted By
                    </Text>
                    <Text>
                      {postedByLabel}{" "}
                      at {formatDateTime(entry.posted_at || "")}
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
