'use client';

import React, { useState, useEffect } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import SimpleLayout from '@/components/layout/SimpleLayout';
import {
  Box,
  Heading,
  Button,
  VStack,
  HStack,
  Text,
  Card,
  CardBody,
  CardHeader,
  Badge,
  Select,
  Input,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  useToast,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Spinner,
  Divider,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  SimpleGrid,
  Tabs,
  TabList,
  TabPanels,
  Tab,
  TabPanel,
  IconButton,
  Tooltip,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  useDisclosure,
  Textarea,
} from '@chakra-ui/react';
import { FiCheck, FiX, FiRefreshCw, FiLock, FiAlertTriangle, FiFileText, FiEye } from 'react-icons/fi';
import cashbankService, { CashBank } from '@/services/cashbankService';
import bankReconciliationService, {
  BankReconciliationSnapshot,
  BankReconciliation,
  ReconciliationDifference,
} from '@/services/bankReconciliationService';

const BankReconciliationPage: React.FC = () => {
  const { token } = useAuth();
  const toast = useToast();

  // State management
  const [accounts, setAccounts] = useState<CashBank[]>([]);
  const [selectedAccount, setSelectedAccount] = useState<number | null>(null);
  const [selectedPeriod, setSelectedPeriod] = useState<string>('');
  const [snapshots, setSnapshots] = useState<BankReconciliationSnapshot[]>([]);
  const [reconciliations, setReconciliations] = useState<BankReconciliation[]>([]);
  const [selectedSnapshot, setSelectedSnapshot] = useState<BankReconciliationSnapshot | null>(null);
  const [selectedReconciliation, setSelectedReconciliation] = useState<BankReconciliation | null>(null);
  const [loading, setLoading] = useState(false);
  const [tabIndex, setTabIndex] = useState(0);

  // Modal states
  const { isOpen: isSnapshotModalOpen, onOpen: onSnapshotModalOpen, onClose: onSnapshotModalClose } = useDisclosure();
  const { isOpen: isReconcileModalOpen, onOpen: onReconcileModalOpen, onClose: onReconcileModalClose } = useDisclosure();
  const { isOpen: isDetailModalOpen, onOpen: onDetailModalOpen, onClose: onDetailModalClose } = useDisclosure();

  // Form states
  const [snapshotNotes, setSnapshotNotes] = useState('');
  const [reconcileNotes, setReconcileNotes] = useState('');
  const [baseSnapshotId, setBaseSnapshotId] = useState<number | null>(null);

  // Load accounts
  useEffect(() => {
    loadAccounts();
  }, [token]);

  // Load data when account/period changes
  useEffect(() => {
    if (selectedAccount) {
      loadSnapshots();
      loadReconciliations();
    }
  }, [selectedAccount]);

  const loadAccounts = async () => {
    try {
      const data = await cashbankService.getCashBankAccounts();
      setAccounts(data.filter((acc: CashBank) => acc.type === 'BANK' && acc.is_active));
    } catch (error) {
      console.error('Error loading accounts:', error);
      toast({
        title: 'Error',
        description: 'Failed to load bank accounts',
        status: 'error',
        duration: 3000,
      });
    }
  };

  const loadSnapshots = async () => {
    if (!selectedAccount) return;
    try {
      setLoading(true);
      const data = await bankReconciliationService.getSnapshots(selectedAccount);
      setSnapshots(data);
    } catch (error) {
      console.error('Error loading snapshots:', error);
    } finally {
      setLoading(false);
    }
  };

  const loadReconciliations = async () => {
    if (!selectedAccount) return;
    try {
      const data = await bankReconciliationService.getReconciliations(selectedAccount);
      setReconciliations(data);
    } catch (error) {
      console.error('Error loading reconciliations:', error);
    }
  };

  const handleGenerateSnapshot = async () => {
    if (!selectedAccount || !selectedPeriod) {
      toast({
        title: 'Error',
        description: 'Please select account and period',
        status: 'error',
        duration: 3000,
      });
      return;
    }

    try {
      setLoading(true);
      await bankReconciliationService.generateSnapshot({
        cash_bank_id: selectedAccount,
        period: selectedPeriod,
        notes: snapshotNotes,
      });

      toast({
        title: 'Success',
        description: 'Snapshot generated successfully',
        status: 'success',
        duration: 3000,
      });

      setSnapshotNotes('');
      onSnapshotModalClose();
      loadSnapshots();
    } catch (error: any) {
      toast({
        title: 'Error',
        description: error.response?.data?.details || 'Failed to generate snapshot',
        status: 'error',
        duration: 5000,
      });
    } finally {
      setLoading(false);
    }
  };

  const handlePerformReconciliation = async () => {
    if (!selectedAccount || !selectedPeriod || !baseSnapshotId) {
      toast({
        title: 'Error',
        description: 'Please select account, period, and base snapshot',
        status: 'error',
        duration: 3000,
      });
      return;
    }

    try {
      setLoading(true);
      const result = await bankReconciliationService.performReconciliation({
        cash_bank_id: selectedAccount,
        period: selectedPeriod,
        base_snapshot_id: baseSnapshotId,
        notes: reconcileNotes,
      });

      toast({
        title: result.is_balanced ? 'Balanced!' : 'Differences Found',
        description: result.is_balanced
          ? 'No differences found - accounts are balanced'
          : `Found ${result.missing_transactions + result.added_transactions + result.modified_transactions} differences`,
        status: result.is_balanced ? 'success' : 'warning',
        duration: 5000,
      });

      setReconcileNotes('');
      setBaseSnapshotId(null);
      onReconcileModalClose();
      loadReconciliations();
      setTabIndex(1); // Switch to reconciliations tab
    } catch (error: any) {
      toast({
        title: 'Error',
        description: error.response?.data?.details || 'Failed to perform reconciliation',
        status: 'error',
        duration: 5000,
      });
    } finally {
      setLoading(false);
    }
  };

  const handleViewReconciliation = async (reconciliation: BankReconciliation) => {
    try {
      const detail = await bankReconciliationService.getReconciliationById(reconciliation.id);
      setSelectedReconciliation(detail);
      onDetailModalOpen();
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to load reconciliation details',
        status: 'error',
        duration: 3000,
      });
    }
  };

  const handleApprove = async (id: number) => {
    try {
      await bankReconciliationService.approveReconciliation(id, 'Approved by manager');
      toast({
        title: 'Success',
        description: 'Reconciliation approved',
        status: 'success',
        duration: 3000,
      });
      loadReconciliations();
      onDetailModalClose();
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to approve reconciliation',
        status: 'error',
        duration: 3000,
      });
    }
  };

  const handleReject = async (id: number, notes: string) => {
    if (!notes) {
      toast({
        title: 'Error',
        description: 'Please provide rejection notes',
        status: 'error',
        duration: 3000,
      });
      return;
    }

    try {
      await bankReconciliationService.rejectReconciliation(id, notes);
      toast({
        title: 'Success',
        description: 'Reconciliation rejected',
        status: 'success',
        duration: 3000,
      });
      loadReconciliations();
      onDetailModalClose();
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to reject reconciliation',
        status: 'error',
        duration: 3000,
      });
    }
  };

  // Generate period options (last 12 months)
  const getPeriodOptions = () => {
    const options = [];
    const now = new Date();
    for (let i = 0; i < 12; i++) {
      const date = new Date(now.getFullYear(), now.getMonth() - i, 1);
      const period = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`;
      options.push(period);
    }
    return options;
  };

  return (
    <SimpleLayout allowedRoles={['admin', 'finance', 'director']}>
      <Box p={6}>
        <VStack spacing={6} align="stretch">
          {/* Header */}
          <Box>
            <Heading size="lg" mb={2}>
              🔄 Bank Reconciliation
            </Heading>
            <Text color="gray.600">
              Internal reconciliation system - Compare snapshots to detect unauthorized changes
            </Text>
          </Box>

          {/* Info Alert */}
          <Alert status="info" borderRadius="md">
            <AlertIcon />
            <Box>
              <AlertTitle fontSize="sm">How it works</AlertTitle>
              <AlertDescription fontSize="xs">
                1. Generate snapshot at period close → 2. Compare with current data → 3. Review differences → 4. Approve/Reject
              </AlertDescription>
            </Box>
          </Alert>

          {/* Account Selection */}
          <Card>
            <CardHeader>
              <Heading size="md">Select Bank Account</Heading>
            </CardHeader>
            <CardBody>
              <HStack spacing={4}>
                <Box flex="1">
                  <Text fontSize="sm" mb={2} fontWeight="medium">
                    Bank Account
                  </Text>
                  <Select
                    placeholder="Select bank account..."
                    value={selectedAccount || ''}
                    onChange={(e) => setSelectedAccount(Number(e.target.value))}
                  >
                    {accounts.map((acc) => (
                      <option key={acc.id} value={acc.id}>
                        {acc.name} ({acc.account_no})
                      </option>
                    ))}
                  </Select>
                </Box>
                <Box flex="1">
                  <Text fontSize="sm" mb={2} fontWeight="medium">
                    Period
                  </Text>
                  <Select
                    placeholder="Select period..."
                    value={selectedPeriod}
                    onChange={(e) => setSelectedPeriod(e.target.value)}
                  >
                    {getPeriodOptions().map((period) => (
                      <option key={period} value={period}>
                        {bankReconciliationService.formatPeriod(period)}
                      </option>
                    ))}
                  </Select>
                </Box>
              </HStack>
            </CardBody>
          </Card>

          {/* Main Content */}
          {selectedAccount && (
            <Tabs index={tabIndex} onChange={setTabIndex}>
              <TabList>
                <Tab>📸 Snapshots</Tab>
                <Tab>🔍 Reconciliations</Tab>
              </TabList>

              <TabPanels>
                {/* Snapshots Tab */}
                <TabPanel>
                  <VStack spacing={4} align="stretch">
                    <HStack justify="space-between">
                      <Heading size="md">Snapshots</Heading>
                      <Button
                        colorScheme="blue"
                        leftIcon={<FiFileText />}
                        onClick={onSnapshotModalOpen}
                        isDisabled={!selectedPeriod}
                      >
                        Generate Snapshot
                      </Button>
                    </HStack>

                    {loading ? (
                      <Box textAlign="center" py={8}>
                        <Spinner size="xl" />
                      </Box>
                    ) : snapshots.length === 0 ? (
                      <Card>
                        <CardBody textAlign="center" py={8}>
                          <Text color="gray.500">No snapshots yet. Generate your first snapshot for the selected period.</Text>
                        </CardBody>
                      </Card>
                    ) : (
                      <Table variant="simple">
                        <Thead>
                          <Tr>
                            <Th>Period</Th>
                            <Th>Snapshot Date</Th>
                            <Th isNumeric>Closing Balance</Th>
                            <Th isNumeric>Transactions</Th>
                            <Th>Status</Th>
                            <Th>Hash</Th>
                          </Tr>
                        </Thead>
                        <Tbody>
                          {snapshots.map((snapshot) => (
                            <Tr key={snapshot.id}>
                              <Td fontWeight="medium">{bankReconciliationService.formatPeriod(snapshot.period)}</Td>
                              <Td fontSize="sm">{bankReconciliationService.formatDate(snapshot.snapshot_date)}</Td>
                              <Td isNumeric fontFamily="mono">
                                {bankReconciliationService.formatCurrency(snapshot.closing_balance)}
                              </Td>
                              <Td isNumeric>{snapshot.transaction_count}</Td>
                              <Td>
                                <HStack>
                                  <Badge colorScheme={snapshot.is_locked ? 'red' : 'green'}>
                                    {snapshot.is_locked ? 'LOCKED' : 'ACTIVE'}
                                  </Badge>
                                  {snapshot.is_locked && <FiLock />}
                                </HStack>
                              </Td>
                              <Td fontSize="xs" fontFamily="mono" color="gray.500">
                                {snapshot.data_hash.substring(0, 8)}...
                              </Td>
                            </Tr>
                          ))}
                        </Tbody>
                      </Table>
                    )}
                  </VStack>
                </TabPanel>

                {/* Reconciliations Tab */}
                <TabPanel>
                  <VStack spacing={4} align="stretch">
                    <HStack justify="space-between">
                      <Heading size="md">Reconciliation History</Heading>
                      <Button
                        colorScheme="purple"
                        leftIcon={<FiRefreshCw />}
                        onClick={onReconcileModalOpen}
                        isDisabled={!selectedPeriod || snapshots.length === 0}
                      >
                        Perform Reconciliation
                      </Button>
                    </HStack>

                    {reconciliations.length === 0 ? (
                      <Card>
                        <CardBody textAlign="center" py={8}>
                          <Text color="gray.500">No reconciliations yet. Perform your first reconciliation.</Text>
                        </CardBody>
                      </Card>
                    ) : (
                      <VStack spacing={3} align="stretch">
                        {reconciliations.map((recon) => (
                          <Card key={recon.id} borderLeft="4px" borderLeftColor={recon.is_balanced ? 'green.400' : 'orange.400'}>
                            <CardBody>
                              <HStack justify="space-between" mb={3}>
                                <VStack align="start" spacing={1}>
                                  <HStack>
                                    <Badge colorScheme={bankReconciliationService.getStatusColor(recon.status)}>
                                      {recon.status}
                                    </Badge>
                                    <Text fontWeight="bold">{recon.reconciliation_number}</Text>
                                  </HStack>
                                  <Text fontSize="sm" color="gray.600">
                                    {bankReconciliationService.formatDate(recon.reconciliation_date)}
                                  </Text>
                                </VStack>
                                <Button size="sm" leftIcon={<FiEye />} onClick={() => handleViewReconciliation(recon)}>
                                  View Details
                                </Button>
                              </HStack>

                              <SimpleGrid columns={4} spacing={4}>
                                <Stat size="sm">
                                  <StatLabel>Base Balance</StatLabel>
                                  <StatNumber fontSize="md">
                                    {bankReconciliationService.formatCurrency(recon.base_balance)}
                                  </StatNumber>
                                </Stat>
                                <Stat size="sm">
                                  <StatLabel>Current Balance</StatLabel>
                                  <StatNumber fontSize="md">
                                    {bankReconciliationService.formatCurrency(recon.current_balance)}
                                  </StatNumber>
                                </Stat>
                                <Stat size="sm">
                                  <StatLabel>Variance</StatLabel>
                                  <StatNumber fontSize="md" color={recon.variance === 0 ? 'green.500' : 'red.500'}>
                                    {bankReconciliationService.formatCurrency(Math.abs(recon.variance))}
                                  </StatNumber>
                                </Stat>
                                <Stat size="sm">
                                  <StatLabel>Differences</StatLabel>
                                  <StatNumber fontSize="md">
                                    {recon.missing_transactions + recon.added_transactions + recon.modified_transactions}
                                  </StatNumber>
                                  <StatHelpText>
                                    {recon.is_balanced ? '✅ Balanced' : '⚠️ Needs Review'}
                                  </StatHelpText>
                                </Stat>
                              </SimpleGrid>
                            </CardBody>
                          </Card>
                        ))}
                      </VStack>
                    )}
                  </VStack>
                </TabPanel>
              </TabPanels>
            </Tabs>
          )}
        </VStack>

        {/* Generate Snapshot Modal */}
        <Modal isOpen={isSnapshotModalOpen} onClose={onSnapshotModalClose}>
          <ModalOverlay />
          <ModalContent>
            <ModalHeader>Generate Snapshot</ModalHeader>
            <ModalCloseButton />
            <ModalBody>
              <VStack spacing={4}>
                <Alert status="info" fontSize="sm">
                  <AlertIcon />
                  This will create a frozen snapshot of all transactions for {bankReconciliationService.formatPeriod(selectedPeriod)}
                </Alert>
                <Box width="full">
                  <Text fontSize="sm" mb={2}>
                    Notes (optional)
                  </Text>
                  <Textarea
                    placeholder="Add notes about this snapshot..."
                    value={snapshotNotes}
                    onChange={(e) => setSnapshotNotes(e.target.value)}
                  />
                </Box>
              </VStack>
            </ModalBody>
            <ModalFooter>
              <Button variant="ghost" mr={3} onClick={onSnapshotModalClose}>
                Cancel
              </Button>
              <Button colorScheme="blue" onClick={handleGenerateSnapshot} isLoading={loading}>
                Generate
              </Button>
            </ModalFooter>
          </ModalContent>
        </Modal>

        {/* Perform Reconciliation Modal */}
        <Modal isOpen={isReconcileModalOpen} onClose={onReconcileModalClose} size="lg">
          <ModalOverlay />
          <ModalContent>
            <ModalHeader>Perform Reconciliation</ModalHeader>
            <ModalCloseButton />
            <ModalBody>
              <VStack spacing={4}>
                <Alert status="warning" fontSize="sm">
                  <AlertIcon />
                  This will compare the base snapshot with current data to detect any changes or discrepancies.
                </Alert>
                <Box width="full">
                  <Text fontSize="sm" mb={2} fontWeight="medium">
                    Select Base Snapshot
                  </Text>
                  <Select
                    placeholder="Choose a snapshot to compare..."
                    value={baseSnapshotId || ''}
                    onChange={(e) => setBaseSnapshotId(Number(e.target.value))}
                  >
                    {snapshots
                      .filter((s) => s.period === selectedPeriod)
                      .map((snapshot) => (
                        <option key={snapshot.id} value={snapshot.id}>
                          {bankReconciliationService.formatDate(snapshot.snapshot_date)} -{' '}
                          {bankReconciliationService.formatCurrency(snapshot.closing_balance)}
                        </option>
                      ))}
                  </Select>
                </Box>
                <Box width="full">
                  <Text fontSize="sm" mb={2}>
                    Notes (optional)
                  </Text>
                  <Textarea
                    placeholder="Add notes about this reconciliation..."
                    value={reconcileNotes}
                    onChange={(e) => setReconcileNotes(e.target.value)}
                  />
                </Box>
              </VStack>
            </ModalBody>
            <ModalFooter>
              <Button variant="ghost" mr={3} onClick={onReconcileModalClose}>
                Cancel
              </Button>
              <Button colorScheme="purple" onClick={handlePerformReconciliation} isLoading={loading}>
                Reconcile
              </Button>
            </ModalFooter>
          </ModalContent>
        </Modal>

        {/* Reconciliation Detail Modal */}
        <Modal isOpen={isDetailModalOpen} onClose={onDetailModalClose} size="6xl">
          <ModalOverlay />
          <ModalContent>
            <ModalHeader>Reconciliation Details</ModalHeader>
            <ModalCloseButton />
            <ModalBody>
              {selectedReconciliation && (
                <VStack spacing={6} align="stretch">
                  {/* Summary */}
                  <SimpleGrid columns={3} spacing={4}>
                    <Stat>
                      <StatLabel>Base Balance</StatLabel>
                      <StatNumber>{bankReconciliationService.formatCurrency(selectedReconciliation.base_balance)}</StatNumber>
                      <StatHelpText>
                        {selectedReconciliation.base_transaction_count} transactions
                      </StatHelpText>
                    </Stat>
                    <Stat>
                      <StatLabel>Current Balance</StatLabel>
                      <StatNumber>{bankReconciliationService.formatCurrency(selectedReconciliation.current_balance)}</StatNumber>
                      <StatHelpText>
                        {selectedReconciliation.current_transaction_count} transactions
                      </StatHelpText>
                    </Stat>
                    <Stat>
                      <StatLabel>Variance</StatLabel>
                      <StatNumber color={selectedReconciliation.variance === 0 ? 'green.500' : 'red.500'}>
                        {bankReconciliationService.formatCurrency(Math.abs(selectedReconciliation.variance))}
                      </StatNumber>
                      <StatHelpText>
                        {selectedReconciliation.transaction_variance >= 0 ? '+' : ''}
                        {selectedReconciliation.transaction_variance} transactions
                      </StatHelpText>
                    </Stat>
                  </SimpleGrid>

                  <Divider />

                  {/* Differences */}
                  {selectedReconciliation.differences && selectedReconciliation.differences.length > 0 ? (
                    <Box>
                      <Heading size="sm" mb={4}>
                        Differences Found ({selectedReconciliation.differences.length})
                      </Heading>
                      <VStack spacing={2} align="stretch">
                        {selectedReconciliation.differences.map((diff) => (
                          <Alert
                            key={diff.id}
                            status={
                              diff.severity === 'CRITICAL'
                                ? 'error'
                                : diff.severity === 'HIGH'
                                ? 'warning'
                                : 'info'
                            }
                          >
                            <AlertIcon />
                            <Box flex="1">
                              <AlertTitle fontSize="sm">
                                {bankReconciliationService.getDifferenceTypeLabel(diff.difference_type)}
                              </AlertTitle>
                              <AlertDescription fontSize="xs">
                                {diff.description}
                                {diff.old_value && diff.new_value && (
                                  <>
                                    <br />
                                    Old: {diff.old_value} → New: {diff.new_value}
                                  </>
                                )}
                              </AlertDescription>
                            </Box>
                            <Badge colorScheme={bankReconciliationService.getSeverityColor(diff.severity)}>
                              {diff.severity}
                            </Badge>
                          </Alert>
                        ))}
                      </VStack>
                    </Box>
                  ) : (
                    <Alert status="success">
                      <AlertIcon />
                      <Box>
                        <AlertTitle>No Differences Found</AlertTitle>
                        <AlertDescription>All transactions match perfectly!</AlertDescription>
                      </Box>
                    </Alert>
                  )}
                </VStack>
              )}
            </ModalBody>
            <ModalFooter>
              <HStack spacing={3}>
                <Button variant="ghost" onClick={onDetailModalClose}>
                  Close
                </Button>
                {selectedReconciliation && selectedReconciliation.status === 'PENDING' && (
                  <>
                    <Button
                      colorScheme="red"
                      leftIcon={<FiX />}
                      onClick={() => {
                        const notes = prompt('Enter rejection notes:');
                        if (notes) handleReject(selectedReconciliation.id, notes);
                      }}
                    >
                      Reject
                    </Button>
                    <Button
                      colorScheme="green"
                      leftIcon={<FiCheck />}
                      onClick={() => handleApprove(selectedReconciliation.id)}
                    >
                      Approve
                    </Button>
                  </>
                )}
              </HStack>
            </ModalFooter>
          </ModalContent>
        </Modal>
      </Box>
    </SimpleLayout>
  );
};

export default BankReconciliationPage;
