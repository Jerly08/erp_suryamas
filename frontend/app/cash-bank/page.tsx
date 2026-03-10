'use client';

import React, { useEffect, useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useTranslation } from '@/hooks/useTranslation';
import { useModulePermissions } from '@/hooks/usePermissions';
import ProtectedModule from '@/components/common/ProtectedModule';
import SimpleLayout from '@/components/layout/SimpleLayout';
import CompanyTypeGuard from '@/components/common/CompanyTypeGuard';
import { DataTable } from '@/components/common/DataTable';
import {
  Box,
  Flex,
  Heading,
  Button,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Text,
  SimpleGrid,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  Card,
  CardBody,
  Badge,
  useToast,
  useDisclosure,
  IconButton,
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  MenuDivider,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  VStack,
  HStack,
  Tooltip,
  useColorModeValue,
  FormControl,
  FormLabel,
  Input,
  Textarea,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  Divider,
} from '@chakra-ui/react';
import { FiPlus, FiDollarSign, FiCreditCard, FiEdit2, FiEye, FiArrowRight, FiTrendingUp, FiTrendingDown, FiMoreVertical, FiTrash2, FiRefreshCw, FiSettings, FiLock, FiUnlock } from 'react-icons/fi';
import cashbankService, { CashBank, BalanceSummary, getAvailableBalance, FrozenHistory } from '@/services/cashbankService';
import accountService from '@/services/accountService';
import CashBankForm from '@/components/cashbank/CashBankForm';
import DepositWithdrawalForm from '@/components/cashbank/DepositWithdrawalForm'; // Used for withdrawal only
import DepositFormImproved from '@/components/cashbank/DepositFormImproved'; // New improved form for deposits
import TransferForm from '@/components/cashbank/TransferForm';
import TransactionHistoryModal from '@/components/cashbank/TransactionHistoryModal';

// Table columns for cash and bank accounts with COA standards
const getAccountColumns = (
  t: any,
  onEdit?: (account: CashBank) => void,
  onView?: (account: CashBank) => void,
  onDeposit?: (account: CashBank) => void,
  onWithdraw?: (account: CashBank) => void,
  onTransfer?: (account: CashBank) => void,
  onDelete?: (account: CashBank) => void,
  onReconcile?: (account: CashBank) => void,
  onFreeze?: (account: CashBank) => void,
  onUnfreeze?: (account: CashBank) => void,
  textColor?: string,
  mutedTextColor?: string
) => [
    {
      header: t('cashBank.accountCode'),
      accessor: ((row: CashBank) => (
        <Box>
          <Text fontFamily="mono" fontWeight="bold" fontSize="sm" color={textColor}>
            {row.code}
          </Text>
          <Text fontSize="xs" color={mutedTextColor}>
            {row.type} Account
          </Text>
        </Box>
      )) as (row: CashBank) => React.ReactNode
    },
    {
      header: t('cashBank.accountName'),
      accessor: ((row: CashBank) => (
        <Box>
          <Text fontWeight="medium" fontSize="sm" color={textColor}>
            {row.name}
          </Text>
          <Text fontSize="xs" color={mutedTextColor}>
            {row.description || t('cashBank.noDescription')}
          </Text>
        </Box>
      )) as (row: CashBank) => React.ReactNode
    },
    {
      header: t('cashBank.accountType'),
      accessor: ((row: CashBank) => (
        <Box textAlign="center">
          <Flex alignItems="center" gap={2} justifyContent="center" mb={1}>
            <Text fontSize="lg">{row.type === 'CASH' ? '💵' : '🏦'}</Text>
            <Badge
              colorScheme={row.type === 'CASH' ? 'green' : 'blue'}
              variant="solid"
              size="sm"
            >
              {row.type}
            </Badge>
          </Flex>
          <Text fontSize="xs" color={mutedTextColor}>
            {row.type === 'CASH' ? t('cashBank.physicalMoney') : t('cashBank.bankAccount')}
          </Text>
        </Box>
      )) as (row: CashBank) => React.ReactNode
    },
    {
      header: t('cashBank.glAccount'),
      accessor: ((row: CashBank) => {
        if (row.account && row.account.code && row.account.name) {
          return (
            <Box>
              <Flex alignItems="center" gap={2} mb={1}>
                <Badge size="sm" colorScheme="blue" variant="outline">
                  {row.account.code}
                </Badge>
                <Text fontSize="sm" fontWeight="medium" color="blue.600" noOfLines={1}>
                  {row.account.name}
                </Text>
              </Flex>
              <Flex alignItems="center" gap={1}>
                <Text fontSize="xs" color="green.600">
                  ✅ {t('cashBank.integratedWithCOA')}
                </Text>
                <Badge size="xs" colorScheme="green" variant="subtle">
                  {t('cashBank.asset')}
                </Badge>
              </Flex>
            </Box>
          );
        }
        return (
          <Box>
            <Flex alignItems="center" gap={2} mb={1}>
              <Badge size="sm" colorScheme="orange" variant="solid">
                {t('cashBank.unlinked')}
              </Badge>
              <Text fontSize="sm" color="orange.700" fontWeight="medium">
                {t('cashBank.noCOALink')}
              </Text>
            </Flex>
            <Flex alignItems="center" gap={1}>
              <Text fontSize="xs" color="red.500">
                ⚠️ {t('cashBank.requiresGLSetup')}
              </Text>
            </Flex>
          </Box>
        );
      }) as (row: CashBank) => React.ReactNode
    },
    {
      header: t('cashBank.bankDetails'),
      accessor: ((row: CashBank) => {
        if (row.type === 'BANK') {
          return (
            <Box>
              <Text fontWeight="medium" fontSize="sm" color="blue.600" mb={1}>
                {row.bank_name || 'Unknown Bank'}
              </Text>
              <Text fontSize="xs" color={textColor} fontFamily="mono">
                Account: {row.account_no || 'N/A'}
              </Text>
              {row.account_holder_name && (
                <Text fontSize="xs" color={mutedTextColor}>
                  Atas Nama: {row.account_holder_name}
                </Text>
              )}
              {row.branch && (
                <Text fontSize="xs" color={mutedTextColor}>
                  Cabang: {row.branch}
                </Text>
              )}
              {!row.account_holder_name && !row.branch && (
                <Text fontSize="xs" color={mutedTextColor}>
                  Electronic Banking
                </Text>
              )}
            </Box>
          );
        }
        return (
          <Box>
            <Text fontSize="sm" color="green.600" fontWeight="medium" mb={1}>
              {t('cashBank.cashStorage')}
            </Text>
            <Text fontSize="xs" color={mutedTextColor}>
              {t('cashBank.physicalCashManagement')}
            </Text>
            <Text fontSize="xs" color={mutedTextColor}>
              {t('cashBank.manualTransactions')}
            </Text>
          </Box>
        );
      }) as (row: CashBank) => React.ReactNode
    },
    {
      header: t('cashBank.currentBalance'),
      accessor: ((row: CashBank) => {
        const isNegative = row.balance < 0;
        const balanceColor = isNegative ? 'red.500' : (row.balance > 0 ? 'green.600' : 'gray.500');
        const hasFrozen = (row.frozen_amount || 0) > 0;
        const availableBalance = getAvailableBalance(row);

        return (
          <Box textAlign="right">
            <Text
              fontWeight="bold"
              color={balanceColor}
              fontSize="sm"
              fontFamily="mono"
            >
              {row.currency} {Math.abs(row.balance).toLocaleString('id-ID')}
              {isNegative && ' (Dr)'}
            </Text>
            {hasFrozen && (
              <>
                <Text fontSize="xs" color="orange.500" fontFamily="mono" mt={1}>
                  Dana Terblokir: {row.currency} {row.frozen_amount.toLocaleString('id-ID')}
                </Text>
                <Text fontSize="xs" color="green.600" fontWeight="bold" fontFamily="mono">
                  Tersedia: {row.currency} {availableBalance.toLocaleString('id-ID')}
                </Text>
              </>
            )}
            {!hasFrozen && (
              <Text fontSize="xs" color={mutedTextColor} mb={1}>
                {isNegative ? `${t('cashBank.overdraft')}` : (row.balance > 0 ? `${t('cashBank.creditBalance')}` : `${t('cashBank.zeroBalance')}`)}
              </Text>
            )}
            <HStack spacing={1} justifyContent="flex-end" mt={1}>
              <Badge
                size="xs"
                colorScheme={isNegative ? 'red' : (row.balance > 0 ? 'green' : 'gray')}
                variant="subtle"
              >
                {isNegative ? t('cashBank.overdraft').toUpperCase() : (row.balance > 0 ? t('cashBank.positive') : t('cashBank.zero'))}
              </Badge>
              {hasFrozen && (
                <Badge size="xs" colorScheme="orange" variant="solid">
                  TERBLOKIR
                </Badge>
              )}
            </HStack>
          </Box>
        );
      }) as (row: CashBank) => React.ReactNode
    },
    {
      header: t('cashBank.status'),
      accessor: ((row: CashBank) => (
        <Box textAlign="center">
          <Badge
            colorScheme={row.is_active ? 'green' : 'red'}
            mb={2}
            variant="solid"
          >
            {row.is_active ? t('cashBank.active') : t('cashBank.inactive')}
          </Badge>
          <Text fontSize="xs" color={row.is_active ? 'green.600' : 'red.500'}>
            {row.is_active ? `🟢 ${t('cashBank.operational')}` : `🔴 ${t('cashBank.suspended')}`}
          </Text>
        </Box>
      )) as (row: CashBank) => React.ReactNode
    },
    {
      header: t('cashBank.actions'),
      accessor: ((row: CashBank) => (
        <Box>
          <Menu>
            <MenuButton
              as={IconButton}
              aria-label="Account actions"
              icon={<FiMoreVertical />}
              size="sm"
              variant="ghost"
              colorScheme="gray"
            />
            <MenuList>
              <MenuItem
                icon={<FiEye />}
                onClick={() => onView?.(row)}
                fontSize="sm"
              >
                {t('cashBank.viewDetailsTransactions')}
              </MenuItem>
              <MenuItem
                icon={<FiEdit2 />}
                onClick={() => onEdit?.(row)}
                fontSize="sm"
              >
                {t('cashBank.editAccountInfo')}
              </MenuItem>
              <MenuDivider />
              <MenuItem
                icon={<FiTrendingUp />}
                onClick={() => onDeposit?.(row)}
                color="green.600"
                fontSize="sm"
                isDisabled={!row.is_active}
              >
                {t('cashBank.makeDeposit')}
              </MenuItem>
              <MenuItem
                icon={<FiTrendingDown />}
                onClick={() => onWithdraw?.(row)}
                color="orange.600"
                fontSize="sm"
                isDisabled={!row.is_active || row.balance <= 0}
              >
                {t('cashBank.makeWithdrawal')}
              </MenuItem>
              <MenuItem
                icon={<FiArrowRight />}
                onClick={() => onTransfer?.(row)}
                color="blue.600"
                fontSize="sm"
                isDisabled={!row.is_active || row.balance <= 0}
              >
                {t('cashBank.transferFunds')}
              </MenuItem>
              <MenuDivider />
              <MenuItem
                icon={<FiTrash2 />}
                onClick={() => onDelete?.(row)}
                color="red.600"
                fontSize="sm"
                isDisabled={row.balance !== 0}
              >
                {t('cashBank.deleteAccount')}
              </MenuItem>
              <MenuDivider />
              <MenuItem
                icon={<FiLock />}
                onClick={() => onFreeze?.(row)}
                color="orange.600"
                fontSize="sm"
                isDisabled={!row.is_active || row.balance <= 0}
              >
                Blokir Dana
              </MenuItem>
              {(row.frozen_amount || 0) > 0 && (
                <MenuItem
                  icon={<FiUnlock />}
                  onClick={() => onUnfreeze?.(row)}
                  color="green.600"
                  fontSize="sm"
                >
                  Buka Blokir Dana
                </MenuItem>
              )}
            </MenuList>
          </Menu>

          {/* Quick Action Buttons */}
          <HStack spacing={1} mt={2} justify="center">
            <Tooltip label={t('cashBank.viewDetails')} fontSize="xs">
              <IconButton
                aria-label="View details"
                icon={<FiEye />}
                size="xs"
                variant="ghost"
                colorScheme="gray"
                onClick={() => onView?.(row)}
              />
            </Tooltip>

            <Tooltip label={t('cashBank.editAccount')} fontSize="xs">
              <IconButton
                aria-label="Edit account"
                icon={<FiEdit2 />}
                size="xs"
                variant="ghost"
                colorScheme="blue"
                onClick={() => onEdit?.(row)}
              />
            </Tooltip>

            {row.is_active && (
              <Tooltip label={t('cashBank.deposit')} fontSize="xs">
                <IconButton
                  aria-label="Make deposit"
                  icon={<FiTrendingUp />}
                  size="xs"
                  variant="ghost"
                  colorScheme="green"
                  onClick={() => onDeposit?.(row)}
                />
              </Tooltip>
            )}

            {row.is_active && row.balance > 0 && (
              <Tooltip label={t('cashBank.transfer')} fontSize="xs">
                <IconButton
                  aria-label="Transfer funds"
                  icon={<FiArrowRight />}
                  size="xs"
                  variant="ghost"
                  colorScheme="orange"
                  onClick={() => onTransfer?.(row)}
                />
              </Tooltip>
            )}
          </HStack>
        </Box>
      )) as (row: CashBank) => React.ReactNode
    }
  ];

const CashBankPage: React.FC = () => {
  const { token } = useAuth();
  const { t } = useTranslation();
  const toast = useToast();
  const { isOpen, onOpen, onClose } = useDisclosure();

  // Permission-based access control
  const { canCreate, canEdit, canDelete, canExport } = useModulePermissions('cash_bank');

  // Color mode values
  const bg = useColorModeValue('white', 'gray.800');
  const textColor = useColorModeValue('gray.800', 'white');
  const mutedTextColor = useColorModeValue('gray.500', 'gray.400');
  const modalContentBg = useColorModeValue('white', 'gray.800');
  const modalFooterBg = useColorModeValue('gray.50', 'gray.700');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const readOnlyBg = useColorModeValue('gray.50', 'gray.700');

  // Tooltip descriptions for cash bank page
  const tooltips = {
    accountType: 'Tipe akun: Cash (kas tunai) untuk uang fisik, Bank (rekening bank) untuk rekening elektronik',
    accountCode: 'Kode unik akun sesuai standar Chart of Accounts (COA)',
    accountName: 'Nama akun kas/bank untuk identifikasi',
    glAccount: 'Akun General Ledger yang terintegrasi untuk pencatatan otomatis di pembukuan',
    balance: 'Saldo terkini akun kas/bank',
    currency: 'Mata uang akun (IDR, USD, dll)',
    bankName: 'Nama bank untuk akun bank',
    accountNo: 'Nomor rekening bank',
    accountHolder: 'Nama pemegang rekening',
    branch: 'Cabang bank tempat rekening dibuka',
    description: 'Deskripsi atau catatan tambahan untuk akun ini',
    isActive: 'Status akun: Active (dapat digunakan) atau Inactive (tidak aktif)',
    deposit: 'Tambah saldo akun dengan transaksi penerimaan uang (deposit)',
    withdrawal: 'Kurangi saldo akun dengan transaksi pengeluaran uang (withdrawal)',
    transfer: 'Transfer dana antar akun kas/bank internal',
    reconcile: 'Rekonsiliasi saldo dengan catatan bank untuk memastikan keakuratan',
  };

  const [accounts, setAccounts] = useState<CashBank[]>([]);
  const [balanceSummary, setBalanceSummary] = useState<BalanceSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedAccount, setSelectedAccount] = useState<CashBank | null>(null);
  const [formMode, setFormMode] = useState<'create' | 'edit'>('create');

  // Withdrawal form states (for withdrawal only)
  const [transactionAccount, setTransactionAccount] = useState<CashBank | null>(null);
  const [transactionMode, setTransactionMode] = useState<'withdrawal'>('withdrawal');

  const {
    isOpen: isDetailModalOpen,
    onOpen: onDetailModalOpen,
    onClose: onDetailModalClose
  } = useDisclosure();

  const {
    isOpen: isTransactionModalOpen,
    onOpen: onTransactionModalOpen,
    onClose: onTransactionModalClose
  } = useDisclosure();

  const {
    isOpen: isTransferModalOpen,
    onOpen: onTransferModalOpen,
    onClose: onTransferModalClose
  } = useDisclosure();

  const {
    isOpen: isTransactionHistoryModalOpen,
    onOpen: onTransactionHistoryModalOpen,
    onClose: onTransactionHistoryModalClose
  } = useDisclosure();

  const {
    isOpen: isDepositModalOpen,
    onOpen: onDepositModalOpen,
    onClose: onDepositModalClose
  } = useDisclosure();

  const {
    isOpen: isReconcileModalOpen,
    onOpen: onReconcileModalOpen,
    onClose: onReconcileModalClose
  } = useDisclosure();

  // Deposit form states
  const [depositAccount, setDepositAccount] = useState<CashBank | null>(null);

  // Reconciliation states
  const [reconcileAccount, setReconcileAccount] = useState<CashBank | null>(null);
  const [reconciling, setReconciling] = useState(false);

  // Frozen fund states
  const {
    isOpen: isFreezeModalOpen,
    onOpen: onFreezeModalOpen,
    onClose: onFreezeModalClose
  } = useDisclosure();
  const [freezeAccount, setFreezeAccount] = useState<CashBank | null>(null);
  const [freezeMode, setFreezeMode] = useState<'freeze' | 'unfreeze'>('freeze');
  const [freezeAmount, setFreezeAmount] = useState<number>(0);
  const [freezeReason, setFreezeReason] = useState<string>('');
  const [freezeLoading, setFreezeLoading] = useState(false);
  const [frozenHistory, setFrozenHistory] = useState<FrozenHistory[]>([]);

  const fetchData = async () => {
    try {
      setLoading(true);
      setError(null);

      const [accountsData, summaryData, postedBalances] = await Promise.all([
        cashbankService.getCashBankAccounts(),
        cashbankService.getBalanceSummary(),
        accountService.getPostedCOABalances(token!)
      ]);

      // Build map of COA posted-only raw balances by account_id
      const ssotMap = new Map<number, number>();
      postedBalances.forEach((row: any) => ssotMap.set(row.account_id, row.raw_balance));

      // Merge posted-only balances into each cash/bank row (by linked GL account id)
      // IMPORTANT: Prefer real-time CashBank balance; only override with SSOT posted balance
      // when SSOT has a non-zero value. This avoids showing 0 right after a deposit/withdrawal
      // before the SSOT materialized view refreshes.
      const merged = (Array.isArray(accountsData) ? accountsData : []).map((acc: CashBank) => {
        const glId = acc.account_id || acc.account?.id;
        if (glId && ssotMap.has(glId)) {
          const ssotVal = ssotMap.get(glId)!;
          if (typeof ssotVal === 'number' && Math.abs(ssotVal) > 0.0000001) {
            const current = acc.balance || 0;
            const diff = Math.abs(ssotVal - current);
            const rel = current !== 0 ? diff / Math.abs(current) : 0;
            // Only override when SSOT value is reasonably close (<= 1 IDR or <= 1% difference)
            if (diff <= 1 || rel <= 0.01) {
              return { ...acc, balance: ssotVal };
            }
          }
        }
        return acc;
      });

      setAccounts(merged);

      // Recompute summary from merged to keep cards consistent
      const total_cash = merged.filter(a => a.type === 'CASH').reduce((s, a) => s + (a.balance || 0), 0);
      const total_bank = merged.filter(a => a.type === 'BANK').reduce((s, a) => s + (a.balance || 0), 0);
      const total_balance = total_cash + total_bank;
      setBalanceSummary({ ...(summaryData || {}), total_cash, total_bank, total_balance, by_account: [], by_currency: {} } as any);
    } catch (err: any) {
      console.error('Error fetching cash bank data:', err);
      const errorMessage = err.response?.data?.details || err.message || 'Failed to fetch cash & bank data';
      setError(errorMessage);
      toast({
        title: 'Error',
        description: errorMessage,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (token) {
      fetchData();
    }
  }, [token]);

  const handleAddAccount = () => {
    setSelectedAccount(null);
    setFormMode('create');
    onOpen();
  };

  const handleEditAccount = (account: CashBank) => {
    setSelectedAccount(account);
    setFormMode('edit');
    onOpen();
  };

  const handleViewAccount = (account: CashBank) => {
    setSelectedAccount(account);
    onDetailModalOpen();
  };

  const handleDeposit = (account: CashBank) => {
    setDepositAccount(account);
    onDepositModalOpen();
  };

  const handleWithdraw = (account: CashBank) => {
    setTransactionAccount(account);
    setTransactionMode('withdrawal');
    onTransactionModalOpen();
  };

  const handleTransfer = (account: CashBank) => {
    setTransactionAccount(account);
    onTransferModalOpen();
  };

  const handleReconcile = (account: CashBank) => {
    setReconcileAccount(account);
    onReconcileModalOpen();
  };

  const handleFreeze = (account: CashBank) => {
    setFreezeAccount(account);
    setFreezeMode('freeze');
    setFreezeAmount(0);
    setFreezeReason('');
    // Load frozen history
    cashbankService.getFrozenHistory(account.id).then(setFrozenHistory).catch(() => setFrozenHistory([]));
    onFreezeModalOpen();
  };

  const handleUnfreeze = (account: CashBank) => {
    setFreezeAccount(account);
    setFreezeMode('unfreeze');
    setFreezeAmount(0);
    setFreezeReason('');
    cashbankService.getFrozenHistory(account.id).then(setFrozenHistory).catch(() => setFrozenHistory([]));
    onFreezeModalOpen();
  };

  const handleFreezeSubmit = async () => {
    if (!freezeAccount || freezeAmount <= 0 || !freezeReason.trim()) {
      toast({
        title: 'Data tidak lengkap',
        description: 'Masukkan jumlah dan alasan pemblokiran/pembukaan blokir',
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    try {
      setFreezeLoading(true);
      if (freezeMode === 'freeze') {
        await cashbankService.freezeAmount(freezeAccount.id, { amount: freezeAmount, reason: freezeReason });
        toast({
          title: 'Dana berhasil diblokir',
          description: `${freezeAccount.currency} ${freezeAmount.toLocaleString('id-ID')} telah diblokir pada ${freezeAccount.name}`,
          status: 'success',
          duration: 4000,
          isClosable: true,
        });
      } else {
        await cashbankService.unfreezeAmount(freezeAccount.id, { amount: freezeAmount, reason: freezeReason });
        toast({
          title: 'Blokir dana berhasil dibuka',
          description: `${freezeAccount.currency} ${freezeAmount.toLocaleString('id-ID')} telah dibuka blokirnya pada ${freezeAccount.name}`,
          status: 'success',
          duration: 4000,
          isClosable: true,
        });
      }
      onFreezeModalClose();
      fetchData();
    } catch (error: any) {
      const errMsg = error.response?.data?.details || error.response?.data?.message || error.message || 'Gagal memproses';
      toast({
        title: freezeMode === 'freeze' ? 'Gagal memblokir dana' : 'Gagal membuka blokir',
        description: errMsg,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setFreezeLoading(false);
    }
  };

  const handleGlobalReconcile = () => {
    setReconcileAccount(null); // Global reconciliation
    onReconcileModalOpen();
  };

  const handleOpenReconciliationPage = () => {
    // Navigate to dedicated reconciliation page
    window.open('/cash-bank-reconciliation', '_blank');
    onReconcileModalClose();
  };

  const handleCheckReconciliationStatus = async () => {
    try {
      setReconciling(true);

      const response = await fetch('/api/v1/dashboard/finance', {
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) throw new Error('Failed to check reconciliation status');

      const result = await response.json();
      const bankRecon = result.data.bank_reconciliation;

      let statusMessage = '';
      let statusColor: 'success' | 'warning' | 'error' | 'info' = 'info';

      switch (bankRecon.status) {
        case 'never_reconciled':
          statusMessage = 'Bank reconciliation has never been performed. Please run reconciliation to ensure data accuracy.';
          statusColor = 'error';
          break;
        case 'needs_attention':
          statusMessage = `Bank reconciliation is overdue (${bankRecon.days_ago} days ago). Please perform reconciliation soon.`;
          statusColor = 'warning';
          break;
        case 'recent':
          statusMessage = `Bank reconciliation was performed ${bankRecon.days_ago} days ago. Status is acceptable.`;
          statusColor = 'warning';
          break;
        case 'up_to_date':
          statusMessage = 'Bank reconciliation is up to date. All accounts are properly reconciled.';
          statusColor = 'success';
          break;
        default:
          statusMessage = 'Reconciliation status is unknown. Please check manually.';
          statusColor = 'info';
      }

      toast({
        title: 'Reconciliation Status',
        description: statusMessage,
        status: statusColor,
        duration: 6000,
        isClosable: true,
      });

    } catch (error: any) {
      toast({
        title: 'Status Check Failed',
        description: error.message || 'Failed to check reconciliation status',
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setReconciling(false);
    }
  };

  const handleTransactionSuccess = () => {
    fetchData(); // Refresh data after successful transaction
  };

  const handleDelete = async (account: CashBank) => {
    if (account.balance !== 0) {
      toast({
        title: t('cashBank.cannotDeleteAccount'),
        description: t('cashBank.accountMustHaveZeroBalance'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
      return;
    }

    if (window.confirm(`${t('cashBank.confirmDelete')} "${account.name}"? ${t('cashBank.cannotBeUndone')}`)) {
      try {
        await cashbankService.deleteCashBankAccount(account.id);
        toast({
          title: t('cashBank.accountDeleted'),
          description: `${t('common.name')} "${account.name}" ${t('cashBank.accountDeletedSuccess')}`,
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
        fetchData();
      } catch (error: any) {
        toast({
          title: t('cashBank.deleteFailed'),
          description: error.response?.data?.details || error.message || t('cashBank.deleteFailed'),
          status: 'error',
          duration: 5000,
          isClosable: true,
        });
      }
    }
  };

  const handleFormSuccess = () => {
    fetchData();
  };

  const handleFixGLLinks = async () => {
    try {
      setLoading(true);
      const result = await cashbankService.fixGLAccountLinks();

      toast({
        title: t('cashBank.glAccountLinksFixed'),
        description: `${t('cashBank.successfullyFixed')} ${result.fixed_count} ${t('cashBank.cashBankAccounts')}`,
        status: 'success',
        duration: 5000,
        isClosable: true,
      });

      // Refresh data to show updated GL links
      fetchData();
    } catch (error: any) {
      toast({
        title: t('cashBank.failedToFixGLLinks'),
        description: error.response?.data?.details || error.message || t('cashBank.errorFixingGLLinks'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  // Separate cash and bank accounts
  const safeAccounts = Array.isArray(accounts) ? accounts : [];
  const cashAccounts = safeAccounts.filter(acc => acc.type === 'CASH' && acc.is_active);
  const bankAccounts = safeAccounts.filter(acc => acc.type === 'BANK' && acc.is_active);

  // Get columns with handlers
  const accountColumns = getAccountColumns(
    t,
    handleEditAccount,
    handleViewAccount,
    handleDeposit,
    handleWithdraw,
    handleTransfer,
    handleDelete,
    handleReconcile,
    handleFreeze,
    handleUnfreeze,
    textColor,
    mutedTextColor
  );

  if (loading) {
    return (
      <ProtectedModule module="cash_bank">
        <CompanyTypeGuard>
          <SimpleLayout>
            <Box>
              <Text>{t('common.loading')}</Text>
            </Box>
          </SimpleLayout>
        </CompanyTypeGuard>
      </ProtectedModule>
    );
  }

  return (
    <ProtectedModule module="cash_bank">
      <CompanyTypeGuard>
        <SimpleLayout>
          <Box bg={bg} minH="100vh" p={6}>
            <VStack spacing={6} align="stretch">
              {/* Header */}
              <Flex justify="space-between" align="center" wrap="wrap" gap={4}>
                <Heading size="lg">{t('cashBank.title')}</Heading>
                <HStack spacing={3}>
                  <Button
                    colorScheme="blue"
                    leftIcon={<FiPlus />}
                    onClick={handleAddAccount}
                  >
                    {t('common.add')} {t('accounts.title')}
                  </Button>
                </HStack>
              </Flex>

              {error && (
                <Alert status="error" mb={6}>
                  <AlertIcon />
                  <AlertTitle mr={2}>Error!</AlertTitle>
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              )}

              {/* COA Integration Information */}
              <Alert status="info" mb={6} borderRadius="lg">
                <AlertIcon />
                <Box>
                  <AlertTitle>{t('cashBank.chartOfAccountsIntegration')}</AlertTitle>
                  <AlertDescription>
                    <Text fontSize="sm" mb={2}>
                      {t('cashBank.integrationDesc')}
                    </Text>
                    <Flex gap={4} alignItems="center" flexWrap="wrap">
                      <Text fontSize="xs" color="blue.600">
                        🔄 {t('cashBank.autoCreatesGL')}
                      </Text>
                      <Text fontSize="xs" color="green.600">
                        📋 {t('cashBank.linksToBalanceSheetShort')}
                      </Text>
                      <Text fontSize="xs" color="purple.600">
                        🔍 {t('cashBank.fullAuditTrailShort')}
                      </Text>
                      <Button
                        as="a"
                        href="/accounts"
                        target="_blank"
                        size="xs"
                        variant="outline"
                        colorScheme="blue"
                        leftIcon={<Text fontSize="xs">🔗</Text>}
                      >
                        {t('cashBank.viewChartOfAccounts')}
                      </Button>
                    </Flex>
                  </AlertDescription>
                </Box>
              </Alert>

              {/* COA Integration Status */}
              {safeAccounts.length > 0 && (
                <Card mb={6} borderLeft="4px" borderLeftColor={safeAccounts.some(acc => !acc.account) ? 'orange.400' : 'green.400'}>
                  <CardBody>
                    <Flex justify="space-between" align="center">
                      <Box>
                        <Text fontSize="sm" fontWeight="medium" mb={1}>
                          {safeAccounts.some(acc => !acc.account) ? `⚠️ ${t('cashBank.coaIntegrationStatusDesc')}` : `✅ ${t('cashBank.coaIntegrationStatusDesc')}`}
                        </Text>
                        <Text fontSize="xs" color="gray.600">
                          {safeAccounts.filter(acc => acc.account).length} of {safeAccounts.length} {t('cashBank.accountsLinkedToCOA')}
                        </Text>
                      </Box>
                      {safeAccounts.some(acc => !acc.account) && (
                        <Button
                          size="xs"
                          colorScheme="orange"
                          variant="outline"
                          onClick={handleFixGLLinks}
                          isLoading={loading}
                        >
                          {t('cashBank.fixMissingLinks')}
                        </Button>
                      )}
                    </Flex>
                  </CardBody>
                </Card>
              )}

              {/* Summary Cards */}
              <SimpleGrid columns={{ base: 1, md: 3, lg: 5 }} spacing={6} mb={8}>
                <Card>
                  <CardBody>
                    <Stat>
                      <StatLabel>{t('cashBank.totalBankBalance')}</StatLabel>
                      <StatNumber color="blue.500">
                        IDR {balanceSummary?.total_bank?.toLocaleString() || '0'}
                      </StatNumber>
                      <StatHelpText>{bankAccounts.length} {t('cashBank.accounts')}</StatHelpText>
                    </Stat>
                  </CardBody>
                </Card>

                <Card>
                  <CardBody>
                    <Stat>
                      <StatLabel>{t('cashBank.totalCashBalance')}</StatLabel>
                      <StatNumber color="green.500">
                        IDR {balanceSummary?.total_cash?.toLocaleString() || '0'}
                      </StatNumber>
                      <StatHelpText>{cashAccounts.length} {t('cashBank.accounts')}</StatHelpText>
                    </Stat>
                  </CardBody>
                </Card>

                <Card>
                  <CardBody>
                    <Stat>
                      <StatLabel>{t('cashBank.totalBalance')}</StatLabel>
                      <StatNumber color="purple.500">
                        IDR {balanceSummary?.total_balance?.toLocaleString() || '0'}
                      </StatNumber>
                      <StatHelpText>{t('cashBank.combinedTotal')}</StatHelpText>
                    </Stat>
                  </CardBody>
                </Card>

                <Card borderLeft="4px" borderLeftColor="orange.400">
                  <CardBody>
                    <Stat>
                      <StatLabel color="orange.600">Dana Terblokir</StatLabel>
                      <StatNumber color="orange.500">
                        IDR {safeAccounts.reduce((sum, acc) => sum + (acc.frozen_amount || 0), 0).toLocaleString('id-ID')}
                      </StatNumber>
                      <StatHelpText>{safeAccounts.filter(a => (a.frozen_amount || 0) > 0).length} akun terblokir</StatHelpText>
                    </Stat>
                  </CardBody>
                </Card>

                <Card borderLeft="4px" borderLeftColor="green.400">
                  <CardBody>
                    <Stat>
                      <StatLabel color="green.600">Saldo Tersedia</StatLabel>
                      <StatNumber color="green.500">
                        IDR {safeAccounts.reduce((sum, acc) => sum + getAvailableBalance(acc), 0).toLocaleString('id-ID')}
                      </StatNumber>
                      <StatHelpText>Total yang bisa digunakan</StatHelpText>
                    </Stat>
                  </CardBody>
                </Card>
              </SimpleGrid>

              {/* Bank Accounts Section */}
              <Box mb={8}>
                <Flex justify="space-between" align="center" mb={4}>
                  <Heading size="md">{t('cashBank.bankAccounts')}</Heading>
                  <Button
                    size="sm"
                    leftIcon={<FiCreditCard />}
                    colorScheme="blue"
                    onClick={handleAddAccount}
                  >
                    {t('cashBank.addBankAccount')}
                  </Button>
                </Flex>
                {bankAccounts.length > 0 ? (
                  <Box bg={bg} borderRadius="lg" overflow="hidden" boxShadow="sm">
                    <DataTable<CashBank>
                      columns={accountColumns}
                      data={bankAccounts}
                      keyField="id"
                      searchable={true}
                      pagination={true}
                      pageSize={5}
                    />
                  </Box>
                ) : (
                  <Card>
                    <CardBody>
                      <Text color={mutedTextColor} textAlign="center" py={8}>
                        {t('cashBank.noBankAccountsFound')}
                      </Text>
                    </CardBody>
                  </Card>
                )}
              </Box>

              {/* Cash Accounts Section */}
              <Box>
                <Flex justify="space-between" align="center" mb={4}>
                  <Heading size="md">{t('cashBank.cashAccounts')}</Heading>
                  <Button
                    size="sm"
                    leftIcon={<FiDollarSign />}
                    colorScheme="green"
                    onClick={handleAddAccount}
                  >
                    {t('cashBank.addCashAccount')}
                  </Button>
                </Flex>
                {cashAccounts.length > 0 ? (
                  <Box bg={bg} borderRadius="lg" overflow="hidden" boxShadow="sm">
                    <DataTable<CashBank>
                      columns={accountColumns}
                      data={cashAccounts}
                      keyField="id"
                      searchable={true}
                      pagination={true}
                      pageSize={5}
                    />
                  </Box>
                ) : (
                  <Card>
                    <CardBody>
                      <Text color={mutedTextColor} textAlign="center" py={8}>
                        {t('cashBank.noCashAccountsFound')}
                      </Text>
                    </CardBody>
                  </Card>
                )}
              </Box>

              {/* Cash Bank Form Modal */}
              <CashBankForm
                isOpen={isOpen}
                onClose={onClose}
                onSuccess={handleFormSuccess}
                account={selectedAccount}
                mode={formMode}
              />

              {/* Withdrawal Modal - Using legacy form for withdrawal only */}
              <DepositWithdrawalForm
                isOpen={isTransactionModalOpen}
                onClose={onTransactionModalClose}
                onSuccess={handleTransactionSuccess}
                account={transactionAccount}
                mode={transactionMode}
              />

              {/* Transfer Modal */}
              <TransferForm
                isOpen={isTransferModalOpen}
                onClose={onTransferModalClose}
                onSuccess={handleTransactionSuccess}
                sourceAccount={transactionAccount}
              />

              {/* Freeze/Unfreeze Modal */}
              <Modal isOpen={isFreezeModalOpen} onClose={onFreezeModalClose} size="xl">
                <ModalOverlay bg="blackAlpha.600" backdropFilter="blur(5px)" />
                <ModalContent bg={modalContentBg}>
                  <ModalHeader
                    bg={freezeMode === 'freeze' ? 'orange.500' : 'green.500'}
                    color="white"
                    borderTopRadius="md"
                  >
                    <Flex alignItems="center" gap={3}>
                      {freezeMode === 'freeze' ? <FiLock size={20} /> : <FiUnlock size={20} />}
                      <VStack align="start" spacing={0}>
                        <Text fontSize="lg" fontWeight="bold">
                          {freezeMode === 'freeze' ? 'Blokir Dana' : 'Buka Blokir Dana'}
                        </Text>
                        <Text fontSize="sm" opacity={0.9}>
                          {freezeAccount?.name} ({freezeAccount?.code})
                        </Text>
                      </VStack>
                    </Flex>
                  </ModalHeader>
                  <ModalCloseButton color="white" />

                  <ModalBody py={6}>
                    {freezeAccount && (
                      <VStack spacing={5} align="stretch">
                        {/* Account Info */}
                        <Card bg={readOnlyBg}>
                          <CardBody py={3}>
                            <SimpleGrid columns={2} spacing={3}>
                              <Box>
                                <Text fontSize="xs" color={mutedTextColor}>Total Saldo</Text>
                                <Text fontWeight="bold" fontFamily="mono">
                                  {freezeAccount.currency} {freezeAccount.balance.toLocaleString('id-ID')}
                                </Text>
                              </Box>
                              <Box>
                                <Text fontSize="xs" color={mutedTextColor}>Dana Terblokir Saat Ini</Text>
                                <Text fontWeight="bold" fontFamily="mono" color="orange.500">
                                  {freezeAccount.currency} {(freezeAccount.frozen_amount || 0).toLocaleString('id-ID')}
                                </Text>
                              </Box>
                              <Box>
                                <Text fontSize="xs" color={mutedTextColor}>Saldo Tersedia</Text>
                                <Text fontWeight="bold" fontFamily="mono" color="green.600">
                                  {freezeAccount.currency} {getAvailableBalance(freezeAccount).toLocaleString('id-ID')}
                                </Text>
                              </Box>
                              <Box>
                                <Text fontSize="xs" color={mutedTextColor}>
                                  {freezeMode === 'freeze' ? 'Maks. Bisa Diblokir' : 'Maks. Bisa Dibuka'}
                                </Text>
                                <Text fontWeight="bold" fontFamily="mono" color="blue.500">
                                  {freezeAccount.currency} {freezeMode === 'freeze'
                                    ? getAvailableBalance(freezeAccount).toLocaleString('id-ID')
                                    : (freezeAccount.frozen_amount || 0).toLocaleString('id-ID')
                                  }
                                </Text>
                              </Box>
                            </SimpleGrid>
                          </CardBody>
                        </Card>

                        {/* Amount Input */}
                        <FormControl isRequired>
                          <FormLabel fontSize="sm">
                            {freezeMode === 'freeze' ? 'Jumlah Dana yang Diblokir' : 'Jumlah Blokir yang Dibuka'}
                          </FormLabel>
                          <NumberInput
                            value={freezeAmount || ''}
                            onChange={(_, val) => setFreezeAmount(isNaN(val) ? 0 : val)}
                            min={0}
                            max={freezeMode === 'freeze' ? getAvailableBalance(freezeAccount) : (freezeAccount.frozen_amount || 0)}
                          >
                            <NumberInputField placeholder="Masukkan jumlah" />
                            <NumberInputStepper>
                              <NumberIncrementStepper />
                              <NumberDecrementStepper />
                            </NumberInputStepper>
                          </NumberInput>
                          {freezeAmount > 0 && (
                            <Text fontSize="xs" color={mutedTextColor} mt={1}>
                              {freezeAccount.currency} {freezeAmount.toLocaleString('id-ID')}
                            </Text>
                          )}
                          {freezeMode === 'freeze' && freezeAmount > getAvailableBalance(freezeAccount) && (
                            <Text fontSize="xs" color="red.500" mt={1}>
                              Jumlah melebihi saldo tersedia
                            </Text>
                          )}
                          {freezeMode === 'unfreeze' && freezeAmount > (freezeAccount.frozen_amount || 0) && (
                            <Text fontSize="xs" color="red.500" mt={1}>
                              Jumlah melebihi dana terblokir
                            </Text>
                          )}
                        </FormControl>

                        {/* Quick Amount Buttons */}
                        <HStack spacing={2} flexWrap="wrap">
                          {[25, 50, 75, 100].map(pct => {
                            const maxVal = freezeMode === 'freeze' ? getAvailableBalance(freezeAccount) : (freezeAccount.frozen_amount || 0);
                            const val = Math.floor(maxVal * pct / 100);
                            return (
                              <Button
                                key={pct}
                                size="xs"
                                variant="outline"
                                colorScheme={freezeMode === 'freeze' ? 'orange' : 'green'}
                                onClick={() => setFreezeAmount(val)}
                              >
                                {pct}%
                              </Button>
                            );
                          })}
                        </HStack>

                        {/* Reason Input */}
                        <FormControl>
                          <FormLabel fontSize="sm">Alasan</FormLabel>
                          <Textarea
                            value={freezeReason}
                            onChange={(e) => setFreezeReason(e.target.value)}
                            placeholder={freezeMode === 'freeze' ? 'Contoh: Dana ditahan untuk jaminan bank garansi' : 'Contoh: Jaminan bank garansi telah selesai'}
                            rows={2}
                          />
                        </FormControl>

                        {/* Preview */}
                        {freezeAmount > 0 && (
                          <Alert status={freezeMode === 'freeze' ? 'warning' : 'success'} borderRadius="md">
                            <AlertIcon />
                            <Box>
                              <AlertTitle fontSize="sm">
                                {freezeMode === 'freeze' ? 'Setelah pemblokiran:' : 'Setelah pembukaan blokir:'}
                              </AlertTitle>
                              <AlertDescription fontSize="xs">
                                Dana terblokir: {freezeAccount.currency} {(
                                  freezeMode === 'freeze'
                                    ? (freezeAccount.frozen_amount || 0) + freezeAmount
                                    : (freezeAccount.frozen_amount || 0) - freezeAmount
                                ).toLocaleString('id-ID')}
                                {' | '}
                                Saldo tersedia: {freezeAccount.currency} {(
                                  freezeMode === 'freeze'
                                    ? getAvailableBalance(freezeAccount) - freezeAmount
                                    : getAvailableBalance(freezeAccount) + freezeAmount
                                ).toLocaleString('id-ID')}
                              </AlertDescription>
                            </Box>
                          </Alert>
                        )}

                        {/* Frozen History */}
                        {frozenHistory.length > 0 && (
                          <Box>
                            <Divider my={2} />
                            <Text fontSize="sm" fontWeight="bold" mb={2}>Riwayat Blokir/Buka Blokir</Text>
                            <Box maxH="150px" overflowY="auto">
                              <Table size="sm" variant="simple">
                                <Thead>
                                  <Tr>
                                    <Th fontSize="xs">Tanggal</Th>
                                    <Th fontSize="xs">Aksi</Th>
                                    <Th fontSize="xs" isNumeric>Jumlah</Th>
                                    <Th fontSize="xs">Alasan</Th>
                                  </Tr>
                                </Thead>
                                <Tbody>
                                  {frozenHistory.slice(0, 10).map((h) => (
                                    <Tr key={h.id}>
                                      <Td fontSize="xs">{new Date(h.created_at).toLocaleDateString('id-ID')}</Td>
                                      <Td>
                                        <Badge size="xs" colorScheme={h.action === 'FREEZE' ? 'orange' : 'green'}>
                                          {h.action === 'FREEZE' ? 'BLOKIR' : 'BUKA'}
                                        </Badge>
                                      </Td>
                                      <Td fontSize="xs" isNumeric fontFamily="mono">
                                        {h.amount.toLocaleString('id-ID')}
                                      </Td>
                                      <Td fontSize="xs" maxW="150px" isTruncated>{h.reason}</Td>
                                    </Tr>
                                  ))}
                                </Tbody>
                              </Table>
                            </Box>
                          </Box>
                        )}
                      </VStack>
                    )}
                  </ModalBody>

                  <ModalFooter bg={modalFooterBg} borderBottomRadius="md">
                    <Button variant="ghost" mr={3} onClick={onFreezeModalClose}>
                      Batal
                    </Button>
                    <Button
                      colorScheme={freezeMode === 'freeze' ? 'orange' : 'green'}
                      onClick={handleFreezeSubmit}
                      isLoading={freezeLoading}
                      isDisabled={
                        freezeAmount <= 0 ||
                        (freezeMode === 'freeze' && freezeAmount > getAvailableBalance(freezeAccount!)) ||
                        (freezeMode === 'unfreeze' && freezeAmount > (freezeAccount?.frozen_amount || 0))
                      }
                      leftIcon={freezeMode === 'freeze' ? <FiLock /> : <FiUnlock />}
                    >
                      {freezeMode === 'freeze' ? 'Blokir Dana' : 'Buka Blokir'}
                    </Button>
                  </ModalFooter>
                </ModalContent>
              </Modal>

              {/* Account Details Modal */}
              <Modal isOpen={isDetailModalOpen} onClose={onDetailModalClose} size="4xl" scrollBehavior="inside">
                <ModalOverlay bg="blackAlpha.600" backdropFilter="blur(10px)" />
                <ModalContent
                  maxH="90vh"
                  maxW={{ base: '95vw', md: '90vw', lg: '70vw' }}
                  mx={4}
                  bg={modalContentBg}
                  display="flex"
                  flexDirection="column"
                >
                  {/* Payment Modal Style Header */}
                  <ModalHeader
                    bg="blue.500"
                    color="white"
                    borderTopRadius="md"
                    py={4}
                    px={6}
                  >
                    <Flex alignItems="center" gap={4}>
                      <Box
                        p={3}
                        bg="whiteAlpha.200"
                        borderRadius="full"
                        fontSize="2xl"
                      >
                        {selectedAccount?.type === 'CASH' ? '💵' : '🏦'}
                      </Box>
                      <VStack align="start" spacing={1}>
                        <Text fontSize="xl" fontWeight="bold">
                          {selectedAccount?.name}
                        </Text>
                        <HStack spacing={3}>
                          <Badge
                            bg="whiteAlpha.200"
                            color="white"
                            px={2}
                            py={1}
                            borderRadius="md"
                            fontSize="xs"
                            fontWeight="medium"
                          >
                            {selectedAccount?.type}
                          </Badge>
                          <Text fontSize="sm" fontFamily="mono" opacity={0.9}>
                            {selectedAccount?.code}
                          </Text>
                          <Badge
                            bg={selectedAccount?.is_active ? 'green.500' : 'red.500'}
                            color="white"
                            px={2}
                            py={1}
                            borderRadius="md"
                            fontSize="xs"
                          >
                            {selectedAccount?.is_active ? t('cashBank.active') : t('cashBank.inactive')}
                          </Badge>
                        </HStack>
                      </VStack>
                    </Flex>
                  </ModalHeader>
                  <ModalCloseButton color="white" />

                  <ModalBody p={6} overflowY="auto" flex="1">
                    {selectedAccount && (
                      <VStack spacing={6} align="stretch">
                        {/* Balance Hero Section */}
                        <Card
                          bg="linear-gradient(135deg, #4facfe 0%, #00f2fe 100%)"
                          color="white"
                          shadow="lg"
                          borderRadius="xl"
                        >
                          <CardBody p={6} textAlign="center">
                            <VStack spacing={4}>
                              <Text fontSize="md" fontWeight="medium" opacity={0.9}>{t('cashBank.currentBalance')}</Text>
                              <Text
                                fontSize="3xl"
                                fontWeight="bold"
                                fontFamily="mono"
                                letterSpacing="tight"
                              >
                                {selectedAccount.currency} {Math.abs(selectedAccount.balance).toLocaleString('id-ID')}
                              </Text>
                              <Badge
                                bg={selectedAccount.balance < 0 ? 'red.500' : selectedAccount.balance > 0 ? 'green.500' : 'gray.500'}
                                color="white"
                                px={3}
                                py={1}
                                borderRadius="full"
                                fontSize="sm"
                              >
                                {selectedAccount.balance < 0 ? `⚠️ ${t('cashBank.overdraft')}` :
                                  selectedAccount.balance > 0 ? `✅ ${t('cashBank.positive')}` : `➖ ${t('cashBank.zeroBalance')}`}
                              </Badge>
                            </VStack>
                          </CardBody>
                        </Card>

                        {/* Information Grid - Compact Layout */}
                        <SimpleGrid columns={{ base: 1, lg: 2 }} spacing={6}>
                          {/* Left Column */}
                          <VStack spacing={4} align="stretch">
                            {/* Account Information */}
                            <Box>
                              <HStack spacing={3} mb={3}>
                                <Text fontSize="xl">📋</Text>
                                <Text fontWeight="bold" fontSize="lg">{t('cashBank.accountInformation')}</Text>
                              </HStack>

                              <VStack spacing={3} align="stretch">
                                <Box>
                                  <Text fontSize="xs" color={mutedTextColor} mb={1} textTransform="uppercase" letterSpacing="wide">{t('cashBank.accountCode')}</Text>
                                  <Text fontWeight="bold" fontFamily="mono" bg={readOnlyBg} px={3} py={2} borderRadius="md">
                                    {selectedAccount.code}
                                  </Text>
                                </Box>
                                <Box>
                                  <Text fontSize="xs" color={mutedTextColor} mb={1} textTransform="uppercase" letterSpacing="wide">{t('cashBank.currency')}</Text>
                                  <HStack spacing={2}>
                                    <Text fontWeight="bold">{selectedAccount.currency}</Text>
                                    <Badge colorScheme="blue" variant="subtle" fontSize="xs">
                                      {t('cashBank.indonesianRupiah')}
                                    </Badge>
                                  </HStack>
                                </Box>
                              </VStack>
                            </Box>

                            {/* Bank Details */}
                            {selectedAccount.type === 'BANK' && (
                              <Box>
                                <HStack spacing={3} mb={3}>
                                  <Text fontSize="xl">🏦</Text>
                                  <Text fontWeight="bold" fontSize="lg">{t('cashBank.bankDetails')}</Text>
                                </HStack>

                                <VStack spacing={3} align="stretch">
                                  <Box>
                                    <Text fontSize="xs" color={mutedTextColor} mb={1} textTransform="uppercase" letterSpacing="wide">{t('cashBank.bankName')}</Text>
                                    <Text bg={readOnlyBg} px={3} py={2} borderRadius="md" color={selectedAccount.bank_name ? textColor : 'orange.600'}>
                                      {selectedAccount.bank_name || t('cashBank.bankNameNotSpecified')}
                                    </Text>
                                  </Box>
                                  <Box>
                                    <Text fontSize="xs" color={mutedTextColor} mb={1} textTransform="uppercase" letterSpacing="wide">{t('cashBank.accountNumber')}</Text>
                                    <Text bg={readOnlyBg} px={3} py={2} borderRadius="md" fontFamily="mono" color={selectedAccount.account_no ? textColor : 'orange.600'}>
                                      {selectedAccount.account_no || t('cashBank.accountNumberNotSpecified')}
                                    </Text>
                                  </Box>
                                  <Box>
                                    <Text fontSize="xs" color={mutedTextColor} mb={1} textTransform="uppercase" letterSpacing="wide">{t('cashBank.atasNama')}</Text>
                                    <Text bg={readOnlyBg} px={3} py={2} borderRadius="md" color={selectedAccount.account_holder_name ? textColor : 'orange.600'}>
                                      {selectedAccount.account_holder_name || t('cashBank.accountHolderNotSpecified')}
                                    </Text>
                                  </Box>
                                  <Box>
                                    <Text fontSize="xs" color={mutedTextColor} mb={1} textTransform="uppercase" letterSpacing="wide">{t('cashBank.cabang')}</Text>
                                    <Text bg={readOnlyBg} px={3} py={2} borderRadius="md" color={selectedAccount.branch ? textColor : 'orange.600'}>
                                      {selectedAccount.branch || t('cashBank.branchNotSpecified')}
                                    </Text>
                                  </Box>
                                </VStack>
                              </Box>
                            )}
                          </VStack>

                          {/* Right Column */}
                          <VStack spacing={4} align="stretch">
                            {/* COA Integration */}
                            <Box>
                              <HStack spacing={3} mb={3}>
                                <Text fontSize="xl">📊</Text>
                                <Text fontWeight="bold" fontSize="lg">{t('cashBank.chartOfAccounts')}</Text>
                              </HStack>

                              {selectedAccount.account ? (
                                <Box bg="green.50" p={4} borderRadius="lg" borderLeft="4px" borderLeftColor="green.400">
                                  <HStack justify="space-between" align="center" mb={2}>
                                    <Badge colorScheme="blue" variant="solid" fontSize="sm" px={3} py={1}>
                                      {selectedAccount.account.code}
                                    </Badge>
                                    <Badge colorScheme="green" variant="solid" fontSize="xs" px={2} py={1}>
                                      {t('cashBank.asset')}
                                    </Badge>
                                  </HStack>
                                  <Text fontWeight="bold" mb={1}>
                                    {selectedAccount.account.name}
                                  </Text>
                                  <Text fontSize="sm" color="green.700">
                                    ✅ {t('cashBank.successfullyIntegrated')}
                                  </Text>
                                </Box>
                              ) : (
                                <Alert status="warning" borderRadius="md" py={3}>
                                  <AlertIcon boxSize={4} />
                                  <Box>
                                    <AlertTitle fontSize="sm">{t('cashBank.coaIntegrationRequired')}</AlertTitle>
                                    <AlertDescription fontSize="xs">
                                      {t('cashBank.linkAccountToCOA')}
                                    </AlertDescription>
                                  </Box>
                                </Alert>
                              )}
                            </Box>

                            {/* Audit Information */}
                            <Box>
                              <HStack spacing={3} mb={3}>
                                <Text fontSize="xl">📅</Text>
                                <Text fontWeight="bold" fontSize="lg">{t('cashBank.auditInformation')}</Text>
                              </HStack>

                              <VStack spacing={3} align="stretch">
                                <Box>
                                  <Text fontSize="xs" color={mutedTextColor} mb={1} textTransform="uppercase" letterSpacing="wide">{t('cashBank.created')}</Text>
                                  <Text fontSize="sm" fontFamily="mono" bg={readOnlyBg} px={3} py={2} borderRadius="md">
                                    {new Date(selectedAccount.created_at).toLocaleDateString('id-ID', {
                                      weekday: 'long',
                                      year: 'numeric',
                                      month: 'long',
                                      day: 'numeric',
                                      hour: '2-digit',
                                      minute: '2-digit'
                                    })}
                                  </Text>
                                </Box>
                                <Box>
                                  <Text fontSize="xs" color={mutedTextColor} mb={1} textTransform="uppercase" letterSpacing="wide">{t('cashBank.lastUpdated')}</Text>
                                  <Text fontSize="sm" fontFamily="mono" bg={readOnlyBg} px={3} py={2} borderRadius="md">
                                    {new Date(selectedAccount.updated_at).toLocaleDateString('id-ID', {
                                      weekday: 'long',
                                      year: 'numeric',
                                      month: 'long',
                                      day: 'numeric',
                                      hour: '2-digit',
                                      minute: '2-digit'
                                    })}
                                  </Text>
                                </Box>
                              </VStack>
                            </Box>
                          </VStack>
                        </SimpleGrid>

                        {/* Quick Actions - Compact */}
                        <Box>
                          <HStack spacing={3} mb={4}>
                            <Text fontSize="xl">⚡</Text>
                            <Text fontWeight="bold" fontSize="lg">{t('cashBank.quickActions')}</Text>
                          </HStack>

                          <SimpleGrid columns={{ base: 2, md: 4 }} spacing={3}>
                            <Button
                              leftIcon={<FiEdit2 />}
                              colorScheme="blue"
                              variant="outline"
                              size="sm"
                              onClick={() => {
                                onDetailModalClose();
                                handleEditAccount(selectedAccount);
                              }}
                            >
                              {t('cashBank.edit')}
                            </Button>

                            <Button
                              leftIcon={<FiEye />}
                              colorScheme="purple"
                              variant="outline"
                              size="sm"
                              onClick={() => {
                                onDetailModalClose();
                                setSelectedAccount(selectedAccount);
                                onTransactionHistoryModalOpen();
                              }}
                            >
                              {t('cashBank.history')}
                            </Button>

                            {selectedAccount.is_active && (
                              <Button
                                leftIcon={<FiTrendingUp />}
                                colorScheme="green"
                                variant="outline"
                                size="sm"
                                onClick={() => {
                                  onDetailModalClose();
                                  handleDeposit(selectedAccount);
                                }}
                              >
                                {t('cashBank.deposit')}
                              </Button>
                            )}

                            {selectedAccount.is_active && selectedAccount.balance > 0 && (
                              <Button
                                leftIcon={<FiArrowRight />}
                                colorScheme="orange"
                                variant="outline"
                                size="sm"
                                onClick={() => {
                                  onDetailModalClose();
                                  handleTransfer(selectedAccount);
                                }}
                              >
                                {t('cashBank.transfer')}
                              </Button>
                            )}
                          </SimpleGrid>
                        </Box>
                      </VStack>
                    )}
                  </ModalBody>

                  {/* Modal Footer with Payment Modal Style */}
                  <ModalFooter
                    bg={modalFooterBg}
                    borderBottomRadius="md"
                    py={4}
                    px={6}
                    borderTop="1px"
                    borderColor={borderColor}
                  >
                    <HStack spacing={3} width="full" justify="space-between">
                      {/* Account Summary Info */}
                      {selectedAccount && (
                        <Box flex="1" display={{ base: 'none', md: 'block' }}>
                          <HStack spacing={4}>
                            <Text fontSize="sm" color={mutedTextColor}>
                              {t('cashBank.balance')}: <Text as="span" fontWeight="bold" color={selectedAccount.balance < 0 ? "red.600" : selectedAccount.balance > 0 ? "green.600" : "gray.600"}>
                                {selectedAccount.currency} {Math.abs(selectedAccount.balance).toLocaleString('id-ID')}
                              </Text>
                            </Text>
                            <Text fontSize="sm" color={mutedTextColor}>
                              {t('cashBank.statusLabel')}: <Text as="span" fontWeight="bold" color={selectedAccount.is_active ? "green.600" : "red.600"}>
                                {selectedAccount.is_active ? t('cashBank.active') : t('cashBank.inactive')}
                              </Text>
                            </Text>
                          </HStack>
                        </Box>
                      )}

                      {/* Action Buttons */}
                      <HStack spacing={3}>
                        <Button
                          variant="outline"
                          onClick={onDetailModalClose}
                          size={{ base: 'sm', md: 'md' }}
                          minW="80px"
                        >
                          {t('cashBank.close')}
                        </Button>
                        <Button
                          colorScheme="blue"
                          leftIcon={<FiEdit2 />}
                          onClick={() => {
                            onDetailModalClose();
                            handleEditAccount(selectedAccount!);
                          }}
                          size={{ base: 'sm', md: 'md' }}
                          minW="120px"
                        >
                          {t('cashBank.editAccountButton')}
                        </Button>
                      </HStack>
                    </HStack>
                  </ModalFooter>
                </ModalContent>
              </Modal>

              {/* Deposit Modal - New Improved Form */}
              <DepositFormImproved
                isOpen={isDepositModalOpen}
                onClose={onDepositModalClose}
                onSuccess={handleTransactionSuccess}
                account={depositAccount}
              />

              {/* Transaction History Modal */}
              <TransactionHistoryModal
                isOpen={isTransactionHistoryModalOpen}
                onClose={onTransactionHistoryModalClose}
                account={selectedAccount}
              />

              {/* Reconciliation Modal */}
              <Modal isOpen={isReconcileModalOpen} onClose={onReconcileModalClose} size="lg">
                <ModalOverlay />
                <ModalContent>
                  <ModalHeader>
                    {reconcileAccount ? `Reconcile ${reconcileAccount.name}` : 'Bank Reconciliation Check'}
                  </ModalHeader>
                  <ModalCloseButton />

                  <ModalBody>
                    <VStack spacing={4} align="stretch">
                      <Alert status="info">
                        <AlertIcon />
                        <Box>
                          <AlertTitle fontSize="sm">Bank Reconciliation Status</AlertTitle>
                          <AlertDescription fontSize="xs">
                            Check the reconciliation status of your cash and bank accounts with the journal system.
                          </AlertDescription>
                        </Box>
                      </Alert>

                      {reconcileAccount ? (
                        <Box p={4} bg="gray.50" borderRadius="md">
                          <Text fontWeight="bold" mb={2}>{reconcileAccount.name}</Text>
                          <HStack justify="space-between" mb={1}>
                            <Text fontSize="sm">Account Type:</Text>
                            <Text fontSize="sm" fontWeight="bold">
                              {reconcileAccount.type} Account
                            </Text>
                          </HStack>
                          <HStack justify="space-between" mb={1}>
                            <Text fontSize="sm">Current Balance:</Text>
                            <Text fontSize="sm" fontWeight="bold">
                              {reconcileAccount.currency} {reconcileAccount.balance.toLocaleString('id-ID')}
                            </Text>
                          </HStack>
                          <HStack justify="space-between">
                            <Text fontSize="sm">Status:</Text>
                            <Badge
                              colorScheme={reconcileAccount.is_active ? 'green' : 'red'}
                              variant="solid"
                              fontSize="xs"
                            >
                              {reconcileAccount.is_active ? 'ACTIVE' : 'INACTIVE'}
                            </Badge>
                          </HStack>
                        </Box>
                      ) : (
                        <Box p={4} bg="blue.50" borderRadius="md" borderLeft="4px" borderLeftColor="blue.400">
                          <Text fontWeight="bold" mb={2} color="blue.800">Global Reconciliation Check</Text>
                          <Text fontSize="sm" color="blue.700">
                            This will check the reconciliation status for all cash and bank accounts in your system.
                          </Text>
                        </Box>
                      )}

                      <Alert status="warning" size="sm">
                        <AlertIcon />
                        <AlertDescription fontSize="xs">
                          <Text fontWeight="bold" mb={1}>Action Required:</Text>
                          To perform actual reconciliation, please use the dedicated
                          <Text as="span" fontWeight="bold" color="blue.600"> Cash & Bank Reconciliation </Text>
                          module which provides advanced reconciliation tools.
                        </AlertDescription>
                      </Alert>

                      <Text fontSize="sm" color="gray.600">
                        Available actions:
                      </Text>

                      <VStack spacing={2} align="stretch">
                        <Button
                          leftIcon={<FiSettings />}
                          variant="outline"
                          colorScheme="purple"
                          onClick={handleOpenReconciliationPage}
                          size="sm"
                        >
                          Open Reconciliation Module
                        </Button>
                        <Button
                          leftIcon={<FiRefreshCw />}
                          variant="outline"
                          onClick={handleCheckReconciliationStatus}
                          isLoading={reconciling}
                          size="sm"
                        >
                          Check Current Status
                        </Button>
                      </VStack>
                    </VStack>
                  </ModalBody>

                  <ModalFooter>
                    <HStack spacing={3}>
                      <Button variant="outline" onClick={onReconcileModalClose}>
                        Close
                      </Button>
                      <Button
                        colorScheme="purple"
                        onClick={handleOpenReconciliationPage}
                      >
                        Go to Reconciliation
                      </Button>
                    </HStack>
                  </ModalFooter>
                </ModalContent>
              </Modal>
            </VStack>
          </Box>
        </SimpleLayout>
      </CompanyTypeGuard>
    </ProtectedModule>
  );
};

export default CashBankPage;
