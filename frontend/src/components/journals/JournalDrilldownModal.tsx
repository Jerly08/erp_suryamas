"use client";

import React, { useState, useEffect, useMemo } from "react";
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  VStack,
  HStack,
  Box,
  Text,
  Badge,
  Button,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  Divider,
  Card,
  CardHeader,
  CardBody,
  Flex,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  Spinner,
  Alert,
  AlertIcon,
  Tooltip,
  IconButton,
  useToast,
  useColorModeValue,
  Tabs,
  TabList,
  TabPanels,
  Tab,
  TabPanel,
  SimpleGrid,
} from "@chakra-ui/react";
import {
  FiActivity,
  FiEye,
  FiEdit,
  FiCheck,
  FiX,
  FiRefreshCw,
  FiClock,
  FiUser,
  FiFileText,
  FiTrendingUp,
  FiAlertCircle,
  FiInfo,
  FiHelpCircle,
} from "react-icons/fi";
import {
  ssotJournalService,
  SSOTJournalEntry,
  SSOTJournalLine,
} from "@/services/ssotJournalService";
import { formatCurrency } from "@/utils/formatters";

interface JournalDrilldownModalProps {
  isOpen: boolean;
  onClose: () => void;
  journalId: number | null;
  onEdit?: (journal: SSOTJournalEntry) => void;
  onPost?: (journal: SSOTJournalEntry) => void;
  onReverse?: (journal: SSOTJournalEntry) => void;
  showRealTimeMonitor?: boolean;
}

interface AccountBalanceInfo {
  account_id: number;
  account_code: string;
  account_name: string;
  current_balance: number;
  balance_type: "DEBIT" | "CREDIT";
  last_updated?: string;
}

interface RealtimeBalanceInfo {
  balance: number;
  balance_type: "DEBIT" | "CREDIT";
  updated_at?: string;
}

const JournalDrilldownModal: React.FC<JournalDrilldownModalProps> = ({
  isOpen,
  onClose,
  journalId,
  onEdit,
  onPost,
  onReverse,
  showRealTimeMonitor = true,
}) => {
  const [journal, setJournal] = useState<SSOTJournalEntry | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [accountBalances, setAccountBalances] = useState<
    Map<number, AccountBalanceInfo>
  >(new Map());
  const [isProcessing, setIsProcessing] = useState(false);

  const toast = useToast();
  const isConnected = false;
  const balanceUpdates = null;
  const getAccountBalance = (
    _accountId: number,
  ): RealtimeBalanceInfo | undefined => undefined;
  const connect = () => {};
  const disconnect = () => {};

  // Color mode values
  const cardBg = useColorModeValue("white", "gray.800");
  const headerBg = useColorModeValue("gray.50", "gray.700");
  const borderColor = useColorModeValue("gray.200", "gray.600");
  const successBg = useColorModeValue("green.50", "green.900");
  const warningBg = useColorModeValue("orange.50", "orange.900");
  const errorBg = useColorModeValue("red.50", "red.900");
  const auditHoverBg = useColorModeValue("gray.50", "gray.700");
  const auditPostedHoverBg = useColorModeValue("green.50", "green.900");
  const auditReversedHoverBg = useColorModeValue("orange.50", "orange.900");

  // Load journal data
  const loadJournal = async () => {
    if (!journalId) return;

    try {
      setIsLoading(true);
      setError(null);

      const journalData = await ssotJournalService.getJournalEntry(journalId);
      setJournal(journalData);

      // Initialize account balances for all lines
      const journalLines = journalData.lines ?? [];
      if (journalLines.length > 0) {
        const balancesMap = new Map<number, AccountBalanceInfo>();

        journalLines.forEach((line: SSOTJournalLine) => {
          if (line.account) {
            // Get real-time balance if available
            const realtimeBalance = getAccountBalance(line.account_id);

            balancesMap.set(line.account_id, {
              account_id: line.account_id,
              account_code: line.account.code,
              account_name: line.account.name,
              current_balance: realtimeBalance?.balance || 0,
              balance_type: realtimeBalance?.balance_type || "DEBIT",
              last_updated: realtimeBalance?.updated_at,
            });
          }
        });

        setAccountBalances(balancesMap);
      }
    } catch (error) {
      console.error("Error loading journal:", error);
      setError(
        error instanceof Error ? error.message : "Failed to load journal entry",
      );
    } finally {
      setIsLoading(false);
    }
  };

  // Load journal when modal opens or journalId changes
  useEffect(() => {
    if (isOpen && journalId) {
      loadJournal();
    }
  }, [isOpen, journalId]);

  // Update account balances from real-time updates
  useEffect(() => {
    if (!showRealTimeMonitor || !journal) return;

    journal.lines?.forEach((line: SSOTJournalLine) => {
      const realtimeBalance = getAccountBalance(line.account_id);
      if (realtimeBalance) {
        setAccountBalances((prev) => {
          const updated = new Map(prev);
          const existing = updated.get(line.account_id);
          if (existing) {
            updated.set(line.account_id, {
              ...existing,
              current_balance: realtimeBalance.balance,
              balance_type: realtimeBalance.balance_type,
              last_updated: realtimeBalance.updated_at,
            });
          }
          return updated;
        });
      }
    });
  }, [balanceUpdates, journal, showRealTimeMonitor, getAccountBalance]);

  // Get status color
  const getStatusColor = (status: string) => {
    switch (status) {
      case "POSTED":
        return "green";
      case "DRAFT":
        return "yellow";
      case "REVERSED":
        return "red";
      default:
        return "gray";
    }
  };

  // Calculate impact on balances
  const calculateBalanceImpact = (
    line: SSOTJournalLine,
  ): { impact: number; description: string } => {
    const debit = line.debit_amount || 0;
    const credit = line.credit_amount || 0;

    if (debit > 0) {
      return {
        impact: debit,
        description: `+${formatCurrency(debit)} (Debit)`,
      };
    } else if (credit > 0) {
      return {
        impact: -credit,
        description: `-${formatCurrency(credit)} (Credit)`,
      };
    }

    return { impact: 0, description: "No impact" };
  };

  // Handle journal actions
  const handlePostJournal = async () => {
    if (!journal) return;

    try {
      setIsProcessing(true);
      const updatedJournal = await ssotJournalService.postJournalEntry(
        journal.id,
      );
      setJournal(updatedJournal);

      toast({
        title: "Journal Entry Approved & Posted",
        description: `Entry ${updatedJournal.entry_number} has been approved and posted successfully`,
        status: "success",
        duration: 5000,
        isClosable: true,
      });

      if (onPost) {
        onPost(updatedJournal);
      }
    } catch (error) {
      toast({
        title: "Error Approving Journal",
        description:
          error instanceof Error
            ? error.message
            : "Failed to approve and post journal entry",
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setIsProcessing(false);
    }
  };

  const handleReverseJournal = async () => {
    if (!journal) return;

    try {
      setIsProcessing(true);
      const reversedJournal = await ssotJournalService.reverseJournalEntry(
        journal.id,
        "Reversed via drilldown modal",
      );

      toast({
        title: "Journal Entry Reversed",
        description: `Entry ${journal.entry_number} has been reversed`,
        status: "warning",
        duration: 5000,
        isClosable: true,
      });

      if (onReverse) {
        onReverse(reversedJournal);
      }

      onClose(); // Close modal after reversing
    } catch (error) {
      toast({
        title: "Error Reversing Journal",
        description:
          error instanceof Error
            ? error.message
            : "Failed to reverse journal entry",
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setIsProcessing(false);
    }
  };

  // Render loading state
  if (isLoading) {
    return (
      <Modal isOpen={isOpen} onClose={onClose} size="xl">
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>Loading Journal Entry</ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            <VStack spacing={4} py={8}>
              <Spinner size="xl" />
              <Text>Loading journal entry details...</Text>
            </VStack>
          </ModalBody>
        </ModalContent>
      </Modal>
    );
  }

  // Render error state
  if (error) {
    return (
      <Modal isOpen={isOpen} onClose={onClose} size="xl">
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>Error Loading Journal</ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            <Alert status="error">
              <AlertIcon />
              <VStack align="start" spacing={2}>
                <Text fontWeight="medium">Failed to load journal entry</Text>
                <Text fontSize="sm">{error}</Text>
              </VStack>
            </Alert>
          </ModalBody>
          <ModalFooter>
            <Button
              onClick={() => loadJournal()}
              leftIcon={<FiRefreshCw />}
              mr={3}
            >
              Retry
            </Button>
            <Button onClick={onClose}>Close</Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    );
  }

  // Render journal details
  if (!journal) {
    return null;
  }

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="6xl">
      <ModalOverlay />
      <ModalContent>
        <ModalHeader>
          <HStack justify="space-between" w="full">
            <VStack align="start" spacing={1}>
              <Text>Journal Entry Details</Text>
              <HStack spacing={3}>
                <Tooltip
                  label="Unique identifier for this journal entry. Used for tracking and referencing."
                  hasArrow
                  placement="top"
                >
                  <Text fontSize="sm" color="gray.500" cursor="help">
                    {journal.entry_number}
                  </Text>
                </Tooltip>
                <Tooltip
                  label={`Status: ${journal.status}. ${journal.status === "DRAFT" ? "Entry is editable and not yet posted to accounts." : journal.status === "POSTED" ? "Entry is finalized and affects account balances." : "Entry has been reversed and no longer affects balances."}`}
                  hasArrow
                  placement="top"
                >
                  <Badge
                    colorScheme={getStatusColor(journal.status)}
                    variant="solid"
                    cursor="help"
                  >
                    {journal.status}
                  </Badge>
                </Tooltip>
                {showRealTimeMonitor && (
                  <Tooltip
                    label={
                      isConnected
                        ? "Real-time balance monitoring is active. Balance updates are live."
                        : "Real-time monitoring is disconnected. Balance data may be outdated."
                    }
                    hasArrow
                    placement="top"
                  >
                    <Badge
                      colorScheme={isConnected ? "green" : "red"}
                      variant="outline"
                      fontSize="xs"
                      cursor="help"
                    >
                      {isConnected ? "Live Updates" : "Offline"}
                    </Badge>
                  </Tooltip>
                )}
              </HStack>
            </VStack>

            <HStack spacing={2}>
              {journal.status === "DRAFT" && onPost && (
                <Tooltip
                  label="Approve this draft journal and post it to the ledger. Once posted, the entry cannot be edited."
                  hasArrow
                  placement="bottom"
                >
                  <Button
                    size="sm"
                    colorScheme="green"
                    leftIcon={<FiCheck />}
                    onClick={handlePostJournal}
                    isLoading={isProcessing}
                  >
                    Approve & Post
                  </Button>
                </Tooltip>
              )}

              {journal.status === "POSTED" && onReverse && (
                <Tooltip
                  label="Create a reversing entry to undo the effects of this posted journal entry. This will restore previous account balances."
                  hasArrow
                  placement="bottom"
                >
                  <Button
                    size="sm"
                    colorScheme="orange"
                    leftIcon={<FiX />}
                    onClick={handleReverseJournal}
                    isLoading={isProcessing}
                  >
                    Reverse
                  </Button>
                </Tooltip>
              )}

              {onEdit && journal.status === "DRAFT" && (
                <Tooltip
                  label="Edit this draft journal entry. You can modify accounts, amounts, and descriptions before posting."
                  hasArrow
                  placement="bottom"
                >
                  <Button
                    size="sm"
                    colorScheme="blue"
                    leftIcon={<FiEdit />}
                    onClick={() => onEdit(journal)}
                  >
                    Edit
                  </Button>
                </Tooltip>
              )}
            </HStack>
          </HStack>
        </ModalHeader>

        <ModalCloseButton />

        <ModalBody>
          <Tabs>
            <TabList>
              <Tooltip
                label="View basic information, financial summary, and entry metadata"
                hasArrow
                placement="top"
              >
                <Tab>Entry Details</Tab>
              </Tooltip>
              <Tooltip
                label="Detailed breakdown of all debit and credit lines in this journal entry"
                hasArrow
                placement="top"
              >
                <Tab>Journal Lines</Tab>
              </Tooltip>
              {showRealTimeMonitor && (
                <Tooltip
                  label="Live account balances and impact analysis from this journal entry"
                  hasArrow
                  placement="top"
                >
                  <Tab>Real-time Balances</Tab>
                </Tooltip>
              )}
              <Tooltip
                label="Complete audit history showing who created, posted, or reversed this entry"
                hasArrow
                placement="top"
              >
                <Tab>Audit Trail</Tab>
              </Tooltip>
            </TabList>

            <TabPanels>
              {/* Entry Details Tab */}
              <TabPanel>
                <VStack spacing={6} align="stretch">
                  {/* Basic Information */}
                  <Card>
                    <CardHeader>
                      <Text fontWeight="semibold">Basic Information</Text>
                    </CardHeader>
                    <CardBody>
                      <SimpleGrid columns={[1, 2, 3]} spacing={4}>
                        <Box>
                          <Text
                            fontSize="sm"
                            fontWeight="medium"
                            color="gray.500"
                          >
                            Entry Number
                          </Text>
                          <Text fontSize="md">{journal.entry_number}</Text>
                        </Box>

                        <Box>
                          <Text
                            fontSize="sm"
                            fontWeight="medium"
                            color="gray.500"
                          >
                            Entry Date
                          </Text>
                          <Text fontSize="md">
                            {new Date(journal.entry_date).toLocaleDateString(
                              "id-ID",
                            )}
                          </Text>
                        </Box>

                        <Box>
                          <Text
                            fontSize="sm"
                            fontWeight="medium"
                            color="gray.500"
                          >
                            Source Type
                          </Text>
                          <Text fontSize="md">{journal.source_type}</Text>
                        </Box>

                        {journal.reference && (
                          <Box>
                            <Text
                              fontSize="sm"
                              fontWeight="medium"
                              color="gray.500"
                            >
                              Reference
                            </Text>
                            <Text fontSize="md">{journal.reference}</Text>
                          </Box>
                        )}

                        <Box>
                          <HStack spacing={1}>
                            <Text
                              fontSize="sm"
                              fontWeight="medium"
                              color="gray.500"
                            >
                              Auto Generated
                            </Text>
                            <Tooltip
                              label="Indicates whether this journal entry was automatically generated by the system (e.g., from sales, purchases) or manually created by a user."
                              hasArrow
                              placement="top"
                            >
                              <Box cursor="help">
                                <FiInfo size={12} color="gray.400" />
                              </Box>
                            </Tooltip>
                          </HStack>
                          <Badge
                            colorScheme={
                              journal.is_auto_generated ? "blue" : "gray"
                            }
                          >
                            {journal.is_auto_generated ? "Yes" : "No"}
                          </Badge>
                        </Box>

                        <Box>
                          <HStack spacing={1}>
                            <Text
                              fontSize="sm"
                              fontWeight="medium"
                              color="gray.500"
                            >
                              Balanced
                            </Text>
                            <Tooltip
                              label="A balanced entry has equal total debits and credits. Unbalanced entries cannot be posted and need correction."
                              hasArrow
                              placement="top"
                            >
                              <Box cursor="help">
                                <FiInfo size={12} color="gray.400" />
                              </Box>
                            </Tooltip>
                          </HStack>
                          <HStack spacing={2}>
                            <Badge
                              colorScheme={
                                journal.is_balanced ? "green" : "red"
                              }
                            >
                              {journal.is_balanced ? "Yes" : "No"}
                            </Badge>
                            {journal.is_balanced ? (
                              <FiCheck size={14} color="green" />
                            ) : (
                              <FiAlertCircle size={14} color="red" />
                            )}
                          </HStack>
                        </Box>
                      </SimpleGrid>

                      <Box mt={4}>
                        <Text
                          fontSize="sm"
                          fontWeight="medium"
                          color="gray.500"
                        >
                          Description
                        </Text>
                        <Text fontSize="md">{journal.description}</Text>
                      </Box>

                      {journal.notes && (
                        <Box mt={4}>
                          <Text
                            fontSize="sm"
                            fontWeight="medium"
                            color="gray.500"
                          >
                            Notes
                          </Text>
                          <Text fontSize="md">{journal.notes}</Text>
                        </Box>
                      )}
                    </CardBody>
                  </Card>

                  {/* Financial Summary */}
                  <Card>
                    <CardHeader>
                      <Text fontWeight="semibold">Financial Summary</Text>
                    </CardHeader>
                    <CardBody>
                      <HStack spacing={8}>
                        <Tooltip
                          label="Total amount of debits in this journal entry. Debits increase asset and expense accounts, decrease liability and revenue accounts."
                          hasArrow
                          placement="top"
                        >
                          <Stat cursor="help">
                            <StatLabel>
                              <HStack spacing={1}>
                                <Text>Total Debit</Text>
                                <FiInfo size={12} color="gray.400" />
                              </HStack>
                            </StatLabel>
                            <StatNumber color="blue.600">
                              {formatCurrency(journal.total_debit)}
                            </StatNumber>
                          </Stat>
                        </Tooltip>

                        <Tooltip
                          label="Total amount of credits in this journal entry. Credits decrease asset and expense accounts, increase liability and revenue accounts."
                          hasArrow
                          placement="top"
                        >
                          <Stat cursor="help">
                            <StatLabel>
                              <HStack spacing={1}>
                                <Text>Total Credit</Text>
                                <FiInfo size={12} color="gray.400" />
                              </HStack>
                            </StatLabel>
                            <StatNumber color="green.600">
                              {formatCurrency(journal.total_credit)}
                            </StatNumber>
                          </Stat>
                        </Tooltip>

                        <Tooltip
                          label={`Difference between total debits and credits. A balanced journal entry has zero difference. ${journal.is_balanced ? "This entry is properly balanced." : "This entry is unbalanced and needs correction before posting."}`}
                          hasArrow
                          placement="top"
                        >
                          <Stat cursor="help">
                            <StatLabel>
                              <HStack spacing={1}>
                                <Text>Difference</Text>
                                <FiInfo size={12} color="gray.400" />
                              </HStack>
                            </StatLabel>
                            <StatNumber
                              color={
                                journal.is_balanced ? "green.600" : "red.600"
                              }
                            >
                              {formatCurrency(
                                Math.abs(
                                  journal.total_debit - journal.total_credit,
                                ),
                              )}
                            </StatNumber>
                            <StatHelpText>
                              <HStack spacing={1}>
                                <Text>
                                  {journal.is_balanced
                                    ? "Balanced"
                                    : "Not Balanced"}
                                </Text>
                                {journal.is_balanced ? (
                                  <FiCheck size={14} color="green" />
                                ) : (
                                  <FiAlertCircle size={14} color="red" />
                                )}
                              </HStack>
                            </StatHelpText>
                          </Stat>
                        </Tooltip>
                      </HStack>
                    </CardBody>
                  </Card>
                </VStack>
              </TabPanel>

              {/* Journal Lines Tab */}
              <TabPanel>
                <Card>
                  <CardHeader>
                    <Text fontWeight="semibold">
                      Journal Lines ({journal.lines?.length || 0})
                    </Text>
                  </CardHeader>
                  <CardBody p={0}>
                    <Table variant="simple">
                      <Thead bg={headerBg}>
                        <Tr>
                          <Tooltip
                            label="Sequential line number within this journal entry"
                            hasArrow
                            placement="top"
                          >
                            <Th cursor="help">Line #</Th>
                          </Tooltip>
                          <Tooltip
                            label="The account affected by this journal line, including account code and name"
                            hasArrow
                            placement="top"
                          >
                            <Th cursor="help">Account</Th>
                          </Tooltip>
                          <Tooltip
                            label="Description of the transaction for this specific line"
                            hasArrow
                            placement="top"
                          >
                            <Th cursor="help">Description</Th>
                          </Tooltip>
                          <Tooltip
                            label="Debit amount for this line. Debits increase assets/expenses, decrease liabilities/equity/revenue"
                            hasArrow
                            placement="top"
                          >
                            <Th cursor="help">Debit</Th>
                          </Tooltip>
                          <Tooltip
                            label="Credit amount for this line. Credits decrease assets/expenses, increase liabilities/equity/revenue"
                            hasArrow
                            placement="top"
                          >
                            <Th cursor="help">Credit</Th>
                          </Tooltip>
                          <Tooltip
                            label="How this journal line will impact the account's balance when posted"
                            hasArrow
                            placement="top"
                          >
                            <Th cursor="help">Balance Impact</Th>
                          </Tooltip>
                        </Tr>
                      </Thead>
                      <Tbody>
                        {journal.lines?.map(
                          (line: SSOTJournalLine, index: number) => {
                            const impact = calculateBalanceImpact(line);

                            return (
                              <Tr key={line.id}>
                                <Td>{line.line_number}</Td>
                                <Td>
                                  <VStack align="start" spacing={1}>
                                    <Text fontSize="sm" fontWeight="medium">
                                      {line.account?.code} -{" "}
                                      {line.account?.name}
                                    </Text>
                                    <Badge size="sm" colorScheme="gray">
                                      {line.account?.type}
                                    </Badge>
                                  </VStack>
                                </Td>
                                <Td>
                                  <Text fontSize="sm">{line.description}</Text>
                                </Td>
                                <Td>
                                  <Text
                                    fontSize="sm"
                                    fontWeight="medium"
                                    color="blue.600"
                                  >
                                    {line.debit_amount
                                      ? formatCurrency(line.debit_amount)
                                      : "-"}
                                  </Text>
                                </Td>
                                <Td>
                                  <Text
                                    fontSize="sm"
                                    fontWeight="medium"
                                    color="green.600"
                                  >
                                    {line.credit_amount
                                      ? formatCurrency(line.credit_amount)
                                      : "-"}
                                  </Text>
                                </Td>
                                <Td>
                                  <Text
                                    fontSize="sm"
                                    fontWeight="medium"
                                    color={
                                      impact.impact >= 0
                                        ? "blue.600"
                                        : "green.600"
                                    }
                                  >
                                    {impact.description}
                                  </Text>
                                </Td>
                              </Tr>
                            );
                          },
                        )}
                      </Tbody>
                    </Table>
                  </CardBody>
                </Card>
              </TabPanel>

              {/* Real-time Balances Tab */}
              {showRealTimeMonitor && (
                <TabPanel>
                  <VStack spacing={4} align="stretch">
                    <Card>
                      <CardHeader>
                        <Flex justify="space-between" align="center">
                          <Text fontWeight="semibold">
                            Current Account Balances
                          </Text>
                          <HStack spacing={2}>
                            <Badge colorScheme={isConnected ? "green" : "red"}>
                              {isConnected ? "Live" : "Offline"}
                            </Badge>
                            {!isConnected && (
                              <Button size="xs" onClick={connect}>
                                Connect
                              </Button>
                            )}
                          </HStack>
                        </Flex>
                      </CardHeader>
                      <CardBody p={0}>
                        <Table variant="simple">
                          <Thead bg={headerBg}>
                            <Tr>
                              <Tooltip
                                label="Account code and name affected by this journal entry"
                                hasArrow
                                placement="top"
                              >
                                <Th cursor="help">Account</Th>
                              </Tooltip>
                              <Tooltip
                                label="Current real-time balance of this account"
                                hasArrow
                                placement="top"
                              >
                                <Th cursor="help">Current Balance</Th>
                              </Tooltip>
                              <Tooltip
                                label="Balance type: DEBIT (positive balance for assets/expenses) or CREDIT (positive balance for liabilities/equity/revenue)"
                                hasArrow
                                placement="top"
                              >
                                <Th cursor="help">Balance Type</Th>
                              </Tooltip>
                              <Tooltip
                                label="Last time the balance was updated in the system"
                                hasArrow
                                placement="top"
                              >
                                <Th cursor="help">Last Updated</Th>
                              </Tooltip>
                              <Tooltip
                                label="How this journal entry will affect the account balance when posted"
                                hasArrow
                                placement="top"
                              >
                                <Th cursor="help">Journal Impact</Th>
                              </Tooltip>
                            </Tr>
                          </Thead>
                          <Tbody>
                            {Array.from(accountBalances.entries()).map(
                              ([accountId, balanceInfo]) => {
                                const journalLine = journal.lines?.find(
                                  (line: SSOTJournalLine) =>
                                    line.account_id === accountId,
                                );
                                const impact = journalLine
                                  ? calculateBalanceImpact(journalLine)
                                  : { impact: 0, description: "No impact" };

                                return (
                                  <Tr key={accountId}>
                                    <Td>
                                      <VStack align="start" spacing={1}>
                                        <Text fontSize="sm" fontWeight="medium">
                                          {balanceInfo.account_code} -{" "}
                                          {balanceInfo.account_name}
                                        </Text>
                                      </VStack>
                                    </Td>
                                    <Td>
                                      <Text
                                        fontSize="sm"
                                        fontWeight="medium"
                                        color="green.600"
                                      >
                                        {formatCurrency(
                                          Math.abs(balanceInfo.current_balance),
                                        )}
                                      </Text>
                                    </Td>
                                    <Td>
                                      <Tooltip
                                        label={`${balanceInfo.balance_type} balance type. ${balanceInfo.balance_type === "DEBIT" ? "DEBIT accounts (Assets, Expenses) have positive balances when they increase." : "CREDIT accounts (Liabilities, Equity, Revenue) have positive balances when they increase."}`}
                                        hasArrow
                                        placement="top"
                                      >
                                        <Badge
                                          colorScheme={
                                            balanceInfo.balance_type === "DEBIT"
                                              ? "blue"
                                              : "green"
                                          }
                                          cursor="help"
                                        >
                                          {balanceInfo.balance_type}
                                        </Badge>
                                      </Tooltip>
                                    </Td>
                                    <Td>
                                      <Text fontSize="xs" color="gray.500">
                                        {balanceInfo.last_updated
                                          ? new Date(
                                              balanceInfo.last_updated,
                                            ).toLocaleString()
                                          : "Not updated"}
                                      </Text>
                                    </Td>
                                    <Td>
                                      <Text
                                        fontSize="sm"
                                        color={
                                          impact.impact >= 0
                                            ? "blue.600"
                                            : "green.600"
                                        }
                                      >
                                        {impact.description}
                                      </Text>
                                    </Td>
                                  </Tr>
                                );
                              },
                            )}
                          </Tbody>
                        </Table>
                      </CardBody>
                    </Card>
                  </VStack>
                </TabPanel>
              )}

              {/* Audit Trail Tab */}
              <TabPanel>
                <Card>
                  <CardHeader>
                    <Text fontWeight="semibold">Audit Trail</Text>
                  </CardHeader>
                  <CardBody>
                    <VStack spacing={4} align="stretch">
                      <Tooltip
                        label="Initial creation of this journal entry. All journal entries start in DRAFT status when created."
                        hasArrow
                        placement="left"
                      >
                        <HStack
                          spacing={4}
                          cursor="help"
                          p={2}
                          borderRadius="md"
                          _hover={{ bg: auditHoverBg }}
                        >
                          <Box color="blue.500">
                            <FiUser />
                          </Box>
                          <VStack align="start" spacing={0}>
                            <Text fontSize="sm" fontWeight="medium">
                              Created by
                            </Text>
                            <Text fontSize="sm" color="gray.600">
                              User ID: {journal.created_by}
                            </Text>
                            <Text fontSize="xs" color="gray.500">
                              {new Date(journal.created_at).toLocaleString()}
                            </Text>
                          </VStack>
                        </HStack>
                      </Tooltip>

                      {journal.posted_by && (
                        <>
                          <Divider />
                          <Tooltip
                            label="Journal entry was posted, making it permanent and updating account balances. Posted entries cannot be edited, only reversed."
                            hasArrow
                            placement="left"
                          >
                            <HStack
                              spacing={4}
                              cursor="help"
                              p={2}
                              borderRadius="md"
                              _hover={{ bg: auditPostedHoverBg }}
                            >
                              <Box color="green.500">
                                <FiCheck />
                              </Box>
                              <VStack align="start" spacing={0}>
                                <Text fontSize="sm" fontWeight="medium">
                                  Posted by
                                </Text>
                                <Text fontSize="sm" color="gray.600">
                                  User ID: {journal.posted_by}
                                </Text>
                                {journal.posted_at && (
                                  <Text fontSize="xs" color="gray.500">
                                    {new Date(
                                      journal.posted_at,
                                    ).toLocaleString()}
                                  </Text>
                                )}
                              </VStack>
                            </HStack>
                          </Tooltip>
                        </>
                      )}

                      {journal.reversed_by && (
                        <>
                          <Divider />
                          <Tooltip
                            label="Journal entry was reversed, creating an opposite entry to undo its effects on account balances. The original entry is preserved for audit purposes."
                            hasArrow
                            placement="left"
                          >
                            <HStack
                              spacing={4}
                              cursor="help"
                              p={2}
                              borderRadius="md"
                              _hover={{ bg: auditReversedHoverBg }}
                            >
                              <Box color="orange.500">
                                <FiX />
                              </Box>
                              <VStack align="start" spacing={0}>
                                <Text fontSize="sm" fontWeight="medium">
                                  Reversed by
                                </Text>
                                <Text fontSize="sm" color="gray.600">
                                  User ID: {journal.reversed_by}
                                </Text>
                                {journal.reversed_at && (
                                  <Text fontSize="xs" color="gray.500">
                                    {new Date(
                                      journal.reversed_at,
                                    ).toLocaleString()}
                                  </Text>
                                )}
                              </VStack>
                            </HStack>
                          </Tooltip>
                        </>
                      )}

                      <Divider />
                      <Tooltip
                        label="Most recent modification timestamp for this journal entry, including any status changes."
                        hasArrow
                        placement="left"
                      >
                        <HStack
                          spacing={4}
                          cursor="help"
                          p={2}
                          borderRadius="md"
                          _hover={{ bg: auditHoverBg }}
                        >
                          <Box color="gray.500">
                            <FiClock />
                          </Box>
                          <VStack align="start" spacing={0}>
                            <Text fontSize="sm" fontWeight="medium">
                              Last updated
                            </Text>
                            <Text fontSize="xs" color="gray.500">
                              {new Date(journal.updated_at).toLocaleString()}
                            </Text>
                          </VStack>
                        </HStack>
                      </Tooltip>
                    </VStack>
                  </CardBody>
                </Card>
              </TabPanel>
            </TabPanels>
          </Tabs>
        </ModalBody>

        <ModalFooter>
          <Button onClick={onClose}>Close</Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export default JournalDrilldownModal;
