'use client';

import React, { useState, useEffect } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useTranslation } from '@/hooks/useTranslation';
import { useModulePermissions } from '@/hooks/usePermissions';
import { parseClosedPeriodError, formatClosedPeriodDate } from '@/utils/errorHandler';
import ProtectedModule from '@/components/common/ProtectedModule';
import SimpleLayout from '@/components/layout/SimpleLayout';
import CompanyTypeGuard from '@/components/common/CompanyTypeGuard';
import Table from '@/components/common/Table';
import {
  Box,
  Flex,
  Heading,
  Button,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  useDisclosure,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  AlertDialog,
  AlertDialogBody,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogContent,
  AlertDialogOverlay,
  useToast,
  FormControl,
  FormLabel,
  Input,
  Select,
  Textarea,
  Switch,
  VStack,
  HStack,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  Text,
  Badge,
  Icon,
  Image,
  Grid,
  GridItem,
  Tooltip,
  useColorModeValue,
} from '@chakra-ui/react';
import { FiPlus, FiEdit, FiTrash2, FiEye, FiDownload, FiMapPin, FiExternalLink, FiX, FiUpload, FiSettings, FiInfo } from 'react-icons/fi';
import { assetService, Asset as BackendAsset } from '@/services/assetService';
import { ASSET_CATEGORIES, DEPRECIATION_METHOD_LABELS, AssetsSummary } from '@/types/asset';
import AssetSummaryComponent from '@/components/assets/AssetSummary';
import InteractiveMapPicker from '@/components/common/InteractiveMapPicker';
import AssetImageUpload from '@/components/assets/AssetImageUpload';
import CurrencyInput from '@/components/common/CurrencyInput';
import { getAssetImageUrl } from '@/utils/imageUrl';
import {
  validateAssetForm,
  getFieldError,
  hasFieldError,
  ValidationError,
  AssetFormData as FormData
} from '@/utils/assetValidation';

// Use the form data interface from validation utils
type AssetFormData = FormData;

const AssetsPage = () => {
  const { token, user } = useAuth();
  const { t, language } = useTranslation();

  // Permission-based access control
  const { canCreate, canEdit, canDelete, canExport } = useModulePermissions('assets');

  const [assets, setAssets] = useState<BackendAsset[]>([]);
  const [summary, setSummary] = useState<AssetsSummary | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingSummary, setIsLoadingSummary] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  // Tooltip descriptions for asset page
  const tooltips = {
    search: 'Cari aset berdasarkan nama, kode, serial number, atau lokasi',
    category: 'Kategori aset: Office Equipment (peralatan kantor), Vehicle (kendaraan), Building (bangunan), dll',
    status: 'Status aset: Active (digunakan), Disposed (dijual/dibuang), Under Maintenance (dalam perbaikan)',
    purchaseDate: 'Tanggal pembelian atau perolehan aset',
    purchasePrice: 'Harga perolehan aset (cost basis)',
    salvageValue: 'Nilai sisa/residu aset setelah masa manfaat habis',
    usefulLife: 'Masa manfaat aset dalam tahun (untuk perhitungan depresiasi)',
    depreciationMethod: 'Metode penyusutan: Straight Line (garis lurus), Declining Balance (saldo menurun)',
    condition: 'Kondisi fisik aset saat ini: Good (baik), Fair (cukup), Poor (buruk)',
    location: 'Lokasi fisik aset berada',
    serialNumber: 'Nomor seri atau identifikasi unik aset',
    assetAccount: 'Akun aset tetap di chart of accounts (opsional)',
    depreciationAccount: 'Akun beban penyusutan di chart of accounts (opsional)',
    notes: 'Catatan tambahan tentang aset (riwayat, spesifikasi, dll)',
  };
  const headingColor = useColorModeValue('gray.800', 'whiteAlpha.900');
  const textColor = useColorModeValue('gray.700', 'whiteAlpha.900');
  const mutedTextColor = useColorModeValue('gray.600', 'whiteAlpha.700');
  const inputBg = useColorModeValue('white', 'whiteAlpha.50');
  const inputBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
  const panelBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const modalBg = useColorModeValue('white', 'gray.800');
  const modalHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const modalFooterBg = useColorModeValue('gray.50', 'whiteAlpha.50');
  const modalBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const helperTextColor = useColorModeValue('gray.500', 'whiteAlpha.600');
  const subtleFieldBg = useColorModeValue('gray.50', 'whiteAlpha.50');
  const infoAlertBg = useColorModeValue('blue.50', 'blue.900');
  const infoAlertBorder = useColorModeValue('blue.200', 'blue.700');
  const infoAlertTitleColor = useColorModeValue('blue.700', 'blue.100');
  const infoAlertTextColor = useColorModeValue('blue.600', 'blue.200');
  const detailPlaceholderBg = useColorModeValue('gray.100', 'whiteAlpha.100');
  const selectedCategoryBg = useColorModeValue('blue.50', 'blue.900');
  const selectedCategoryColor = useColorModeValue('blue.700', 'blue.200');
  const defaultCategoryBg = useColorModeValue('gray.50', 'whiteAlpha.50');
  const emptyStateBg = useColorModeValue('gray.50', 'whiteAlpha.50');
  const emptyStateBorder = useColorModeValue('gray.300', 'whiteAlpha.300');
  const tooltipBg = useColorModeValue('gray.700', 'gray.700');
  const entryCardBg = useColorModeValue('white', 'whiteAlpha.50');
  const entryCardBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const purchaseEntryBg = useColorModeValue('blue.50', 'blue.900');
  const openingEntryBg = useColorModeValue('green.50', 'green.900');
  const purchaseEntryTextColor = useColorModeValue('blue.700', 'blue.100');
  const openingEntryTextColor = useColorModeValue('green.700', 'green.100');
  const openingAlertBg = useColorModeValue('green.50', 'green.900');
  const openingAlertBorder = useColorModeValue('green.200', 'green.700');
  const openingAlertTitleColor = useColorModeValue('green.700', 'green.100');
  const openingAlertTextColor = useColorModeValue('green.600', 'green.200');
  const fieldFocusStyles = {
    borderColor: 'blue.400',
    boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)',
  };
  const themedFieldStyles = {
    bg: inputBg,
    borderColor: inputBorderColor,
    color: textColor,
    _hover: { borderColor: inputBorderColor },
    _focusVisible: fieldFocusStyles,
  };
  const readonlyFieldStyles = {
    ...themedFieldStyles,
    bg: subtleFieldBg,
  };

  // Filters & pagination
  const [searchTerm, setSearchTerm] = useState('');
  const [filterCategory, setFilterCategory] = useState('');
  const [filterStatus, setFilterStatus] = useState('');
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);

  const [isModalOpen, setIsModalOpen] = useState(false);
  const [selectedAsset, setSelectedAsset] = useState<BackendAsset | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [validationErrors, setValidationErrors] = useState<ValidationError[]>([]);
  const [isMapPickerOpen, setIsMapPickerOpen] = useState(false);

  // Image upload states
  const [pendingUpload, setPendingUpload] = useState<{ assetId: number, file: File } | null>(null);
  const { isOpen: isAlertOpen, onOpen: onAlertOpen, onClose: onAlertClose } = useDisclosure();

  // Detail view states
  const [detailAsset, setDetailAsset] = useState<BackendAsset | null>(null);
  const { isOpen: isDetailOpen, onOpen: onDetailOpen, onClose: onDetailClose } = useDisclosure();

  // Category management states
  const [isCategoryModalOpen, setIsCategoryModalOpen] = useState(false);

  // Recalculate depreciation (detail modal)
  const [recalcDate, setRecalcDate] = useState('');
  const [recalcResult, setRecalcResult] = useState<null | {
    current_book_value: number;
    accumulated_depreciation: number;
    depreciation_method: string;
  }>(null);
  const [isRecalcLoading, setIsRecalcLoading] = useState(false);
  // Database categories (with code) and merged name list for the dropdown
  const [dbCategories, setDbCategories] = useState<{ id: number; code: string; name: string; is_active: boolean }[]>([]);
  const [customCategories, setCustomCategories] = useState<string[]>([...ASSET_CATEGORIES]);
  const [newCategoryName, setNewCategoryName] = useState('');
  const [editingCategoryIndex, setEditingCategoryIndex] = useState<number | null>(null);
  const [newCategoryCode, setNewCategoryCode] = useState('');

  // Account management states (only for optional fixed asset and depreciation accounts)
  const [fixedAssetAccounts, setFixedAssetAccounts] = useState<any[]>([]);
  const [depreciationAccounts, setDepreciationAccounts] = useState<any[]>([]);
  const [accumulatedDepreciationAccounts, setAccumulatedDepreciationAccounts] = useState<any[]>([]);
  const [bankAccounts, setBankAccounts] = useState<any[]>([]);
  const [liabilityAccounts, setLiabilityAccounts] = useState<any[]>([]);
  const [isLoadingAccounts, setIsLoadingAccounts] = useState(false);

  // Form state
  const [formData, setFormData] = useState<AssetFormData>({
    name: '',
    category: '',
    status: 'ACTIVE',
    purchaseDate: '',
    purchasePrice: 0,
    salvageValue: 0,
    usefulLife: 1,
    depreciationMethod: 'STRAIGHT_LINE',
    isActive: true,
    notes: '',
    location: '',
    coordinates: '',
    serialNumber: '',
    condition: 'Good',
    openingAccumulatedDepreciation: 0,
    entryType: 'purchase',
    paymentMethod: 'CASH',
  });

  // Fetch assets from API
  const fetchAssets = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const response = await assetService.getAssets();
      setAssets(response.data || []);
    } catch (error: any) {
      console.error('Error fetching assets:', error);
      setError(error.response?.data?.message || t('assets.messages.fetchError'));
      toast({
        title: t('common.error'),
        description: t('assets.messages.fetchError'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setIsLoading(false);
    }
  };

  // Fetch asset summary
  const fetchAssetsSummary = async () => {
    try {
      setIsLoadingSummary(true);
      const response = await assetService.getAssetsSummary();
      setSummary(response.data);
    } catch (error: any) {
      console.error('Error fetching assets summary:', error);
      // Don't show error toast for summary as it's not critical
    } finally {
      setIsLoadingSummary(false);
    }
  };

  // Fetch accounts for dropdowns (only optional accounts for manual asset entry)
  const fetchAccounts = async () => {
    try {
      setIsLoadingAccounts(true);
      const [fixedAssetAccountsRes, depreciationAccountsRes, accumDepAccountsRes, bankAccountsRes, liabilityAccountsRes] = await Promise.all([
        assetService.getFixedAssetAccounts(),
        assetService.getDepreciationExpenseAccounts(),
        assetService.getAccumulatedDepreciationAccounts(),
        assetService.getBankAccounts(),
        assetService.getLiabilityAccounts(),
      ]);

      setFixedAssetAccounts(fixedAssetAccountsRes.data || []);
      setDepreciationAccounts(depreciationAccountsRes.data || []);
      setAccumulatedDepreciationAccounts(accumDepAccountsRes.data || []);
      setBankAccounts(bankAccountsRes.data || []);
      setLiabilityAccounts(liabilityAccountsRes.data || []);
    } catch (error: any) {
      console.error('Error fetching accounts:', error);
      // Don't show error toast for accounts as it's not critical
    } finally {
      setIsLoadingAccounts(false);
    }
  };

  // Fetch categories from database
  const fetchCategories = async () => {
    try {
      const response = await assetService.getAssetCategories();
      const cats = (response.data || []).map((c: any) => ({
        id: c.id,
        code: c.code || '',
        name: c.name,
        is_active: c.is_active !== false
      }));
      setDbCategories(cats);

      // Extract category names from response
      const categoryNames = cats.map((cat: any) => cat.name) || [];

      // Merge with default categories (avoid duplicates)
      const allCategories = [...new Set([...ASSET_CATEGORIES, ...categoryNames])];
      setCustomCategories(allCategories);
    } catch (error: any) {
      console.error('Error fetching categories:', error);
      // Fallback to default categories only
      setDbCategories([]);
      setCustomCategories([...ASSET_CATEGORIES]);
    }
  };

  // Load assets on component mount
  useEffect(() => {
    if (token) {
      fetchAssets();
      fetchAssetsSummary();
      fetchAccounts();
      fetchCategories(); // Load categories from database
    }
  }, [token]);

  // Handle form submission for create/update
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    // Validate form data (use dynamic categories loaded from DB)
    const errors = validateAssetForm(formData, customCategories);
    setValidationErrors(errors);

    if (errors.length > 0) {
      toast({
        title: t('assets.messages.validationError'),
        description: t('assets.messages.validationErrorDesc'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      const currentUserId = user?.id ? parseInt(user.id, 10) : NaN;
      if (!formData.id && !Number.isFinite(currentUserId)) {
        throw new Error('User login tidak valid. Silakan login ulang lalu coba lagi.');
      }

      // Transform form data to match backend API
      const apiData = {
        // Note: code is auto-generated by backend if not provided
        name: formData.name,
        category: formData.category,
        status: formData.status,
        purchase_date: formData.purchaseDate.includes('T') ? formData.purchaseDate : `${formData.purchaseDate}T00:00:00Z`,
        purchase_price: formData.purchasePrice,
        salvage_value: formData.salvageValue || 0,
        useful_life: formData.usefulLife,
        depreciation_method: formData.depreciationMethod,
        is_active: formData.isActive !== false,
        notes: formData.notes || '',
        location: formData.location || '',
        coordinates: formData.coordinates || '',
        serial_number: formData.serialNumber || '',
        condition: formData.condition || 'Good',
        asset_account_id: formData.assetAccountId,
        depreciation_account_id: formData.depreciationAccountId,
        accumulated_depreciation_account_id: formData.accumulatedDepreciationAccountId,
        opening_accumulated_depreciation: formData.entryType === 'opening'
          ? (formData.openingAccumulatedDepreciation || 0)
          : 0,
        payment_method: formData.entryType === 'opening' ? 'SALDO_AWAL' : (formData.paymentMethod || 'CASH'),
        payment_account_id: formData.entryType === 'purchase' && (formData.paymentMethod === 'CASH' || formData.paymentMethod === 'BANK') ? formData.paymentAccountId : undefined,
        credit_account_id: formData.entryType === 'purchase' && formData.paymentMethod === 'CREDIT' ? formData.creditAccountId : undefined,
        user_id: currentUserId,
      };

      if (formData.id) {
        await assetService.updateAsset(formData.id, apiData);
        toast({
          title: t('common.buttons.update'),
          description: t('assets.messages.updateSuccess'),
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      } else {
        await assetService.createAsset(apiData);
        toast({
          title: t('common.buttons.create'),
          description: t('assets.messages.createSuccess'),
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      }

      // Refresh assets list and summary
      await Promise.all([fetchAssets(), fetchAssetsSummary()]);

      // Close modal and reset form
      handleCloseModal();
    } catch (error: any) {
      let errorMsg = error.response?.data?.details || error.response?.data?.message || `Error ${formData.id ? 'updating' : 'creating'} asset`;

      // Handle specific duplicate code error
      if (errorMsg.includes('duplicate key value violates unique constraint') ||
        errorMsg.includes('uni_assets_code') ||
        errorMsg.includes('SQLSTATE 23505')) {
        errorMsg = 'An asset with this information already exists. Please check the asset name and category, then try again.';
      }

      setError(errorMsg);
      console.error('Error submitting asset:', error);

      toast({
        title: `Error ${formData.id ? 'Updating' : 'Creating'} Asset`,
        description: errorMsg,
        status: 'error',
        duration: 7000, // Longer duration for error messages
        isClosable: true,
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  // Handle asset deletion
  const handleDelete = async (id: number) => {
    if (!window.confirm(t('assets.messages.confirmDelete'))) {
      return;
    }

    try {
      setError(null);
      await assetService.deleteAsset(id);
      toast({
        title: t('common.delete'),
        description: t('assets.messages.deleteSuccess'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

      // Refresh assets list and summary
      await Promise.all([fetchAssets(), fetchAssetsSummary()]);
    } catch (error: any) {
      const rawMsg = error.response?.data?.error || error.response?.data?.details || error.response?.data?.message || error.message || '';
      const closedPeriod = parseClosedPeriodError(rawMsg);
      if (closedPeriod) {
        const desc = t('common.messages.toast.closedPeriodErrorDesc', { date: formatClosedPeriodDate(closedPeriod.date, language) });
        setError(desc);
        toast({
          title: t('common.messages.toast.closedPeriodError'),
          description: desc,
          status: 'error',
          duration: 5000,
          isClosable: true,
        });
      } else {
        const errorMsg = rawMsg || t('assets.messages.deleteError');
        setError(errorMsg);
        console.error('Error deleting asset:', error);
        toast({
          title: t('common.error'),
          description: errorMsg,
          status: 'error',
          duration: 5000,
          isClosable: true,
        });
      }
    }
  };

  // Reset form data
  const resetForm = () => {
    setFormData({
      name: '',
      category: '',
      status: 'ACTIVE',
      purchaseDate: '',
      purchasePrice: 0,
      salvageValue: 0,
      usefulLife: 1,
      depreciationMethod: 'STRAIGHT_LINE',
      isActive: true,
      notes: '',
      location: '',
      coordinates: '',
      serialNumber: '',
      condition: 'Good',
      openingAccumulatedDepreciation: 0,
      entryType: 'purchase',
      paymentMethod: 'CASH',
    });
  };

  // Close modal and reset
  const handleCloseModal = () => {
    setIsModalOpen(false);
    setSelectedAsset(null);
    setError(null);
    setValidationErrors([]);
    resetForm();
  };

  // Open modal for creating a new asset
  const handleCreate = () => {
    setSelectedAsset(null);
    resetForm();
    setIsModalOpen(true);
  };

  // Open modal for editing an existing asset
  const handleEdit = (asset: BackendAsset) => {
    setSelectedAsset(asset);
    
    // Ensure preloaded accounts are in the dropdown lists so they don't appear empty
    // This is crucial if a saved account was filtered out by API type/category constraints
    if (asset.asset_account_id && asset.asset_account) {
      setFixedAssetAccounts((prev: any[]) => {
        if (!prev.some(a => a.id === asset.asset_account_id)) {
          return [...prev, asset.asset_account];
        }
        return prev;
      });
    }
    
    if (asset.depreciation_account_id && asset.depreciation_account) {
      // Only reject accounts that are clearly accumulated depreciation (1202-xxx)
      // Any other account type is acceptable since the user explicitly selected it
      const acc = asset.depreciation_account;
      const accCode = (acc.code || '').toString();
      const accName = (acc.name || '').toLowerCase();
      const isAccumulatedDepreciation = 
        accCode.startsWith('1202') || accName.includes('akumulasi') || accName.includes('accumulated');
      
      if (!isAccumulatedDepreciation) {
        setDepreciationAccounts((prev: any[]) => {
          if (!prev.some(a => a.id === asset.depreciation_account_id)) {
            return [...prev, acc];
          }
          return prev;
        });
      } else {
        // 1202-xxx account incorrectly stored as depreciation expense - don't inject it
        console.warn(`⚠️ Asset ${asset.code}: depreciation_account_id points to accumulated depreciation account (${accCode} - ${acc.name}). Not injecting.`);
      }
    }
    
    if (asset.accumulated_depreciation_account_id && asset.accumulated_depreciation_account) {
      setAccumulatedDepreciationAccounts((prev: any[]) => {
        if (!prev.some(a => a.id === asset.accumulated_depreciation_account_id)) {
          return [...prev, asset.accumulated_depreciation_account];
        }
        return prev;
      });
    }

    // Determine if the stored depreciation_account_id is valid
    // (it must point to a depreciation expense account, not an accumulated depreciation account)
    let validDepreciationAccountId = asset.depreciation_account_id;
    if (asset.depreciation_account_id && asset.depreciation_account) {
      const depAcc = asset.depreciation_account;
      const depCode = (depAcc.code || '').toString();
      const depName = (depAcc.name || '').toLowerCase();
      const isWrongType = depCode.startsWith('1202') || depName.includes('akumulasi') || depName.includes('accumulated');
      if (isWrongType) {
        console.warn(`⚠️ Asset ${asset.code}: depreciation_account_id=${asset.depreciation_account_id} points to accumulated depreciation account (${depCode} - ${depAcc.name}). Resetting to undefined.`);
        validDepreciationAccountId = undefined;
      }
    }

    // Transform backend data to form format
    setFormData({
      id: asset.id,
      code: asset.code,
      name: asset.name,
      category: asset.category,
      status: asset.status as 'PENDING' | 'ACTIVE' | 'INACTIVE' | 'SOLD',
      purchaseDate: asset.purchase_date.split('T')[0], // Convert to YYYY-MM-DD
      purchasePrice: asset.purchase_price,
      salvageValue: asset.salvage_value,
      usefulLife: asset.useful_life,
      depreciationMethod: asset.depreciation_method as 'STRAIGHT_LINE' | 'DECLINING_BALANCE',
      isActive: asset.is_active,
      notes: asset.notes,
      location: asset.location || '',
      coordinates: asset.coordinates || '',
      serialNumber: asset.serial_number || '',
      condition: asset.condition || 'Good',
      assetAccountId: asset.asset_account_id,
      depreciationAccountId: validDepreciationAccountId,
      accumulatedDepreciationAccountId: asset.accumulated_depreciation_account_id,
      openingAccumulatedDepreciation: asset.opening_accumulated_depreciation || 0
    });
    setIsModalOpen(true);
  };

  // Handle form input changes
  // Suggest accounts based on category name
  const suggestAccountsForCategory = (categoryName: string) => {
    if (!categoryName) return { assetId: formData.assetAccountId, depId: formData.depreciationAccountId };
    const name = categoryName.toLowerCase();
    let fa = fixedAssetAccounts.find(acc => acc.code?.startsWith('1502') || /bangunan|building/.test(acc.name?.toLowerCase() || ''))
      || fixedAssetAccounts.find(acc => acc.code?.startsWith('1503') && /vehicle|kendaraan/.test(name))
      || fixedAssetAccounts.find(acc => acc.code?.startsWith('1504') && /furniture/.test(name))
      || fixedAssetAccounts.find(acc => acc.code?.startsWith('1505') && /(computer|it)/.test(name))
      || fixedAssetAccounts.find(acc => acc.code?.startsWith('1501') && /(machinery|mesin)/.test(name))
      || fixedAssetAccounts[0];
    let dep = depreciationAccounts.find(acc => /depreciation|penyusutan/.test((acc.name || '').toLowerCase())) || depreciationAccounts[0];
    return { assetId: fa?.id, depId: dep?.id };
  };

  const handleInputChange = (field: keyof AssetFormData, value: any) => {
    setFormData(prev => ({
      ...prev,
      [field]: value
    }));

    // Auto-suggest accounts when category changes and no account selected yet
    if (field === 'category') {
      const { assetId, depId } = suggestAccountsForCategory(value as string);
      setFormData(prev => ({
        ...prev,
        assetAccountId: prev.assetAccountId || assetId,
        depreciationAccountId: prev.depreciationAccountId || depId,
      }));
    }
  };

  // Format currency for display
  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount);
  };

  // Calculate current book value
  const calculateBookValue = (asset: BackendAsset) => {
    return asset.purchase_price - asset.accumulated_depreciation;
  };

  // Get status badge color
  const getStatusColor = (status: string) => {
    switch (status) {
      case 'ACTIVE':
        return 'green';
      case 'INACTIVE':
        return 'gray';
      case 'SOLD':
        return 'red';
      default:
        return 'gray';
    }
  };


  // Table columns definition
  const columns = [
    {
      header: t('assets.table.code'),
      accessor: (asset: BackendAsset) => (
        <Text
          fontWeight="medium"
          color="blue.600"
          fontSize="sm"
          whiteSpace="nowrap"
        >
          {asset.code}
        </Text>
      )
    },
    {
      header: t('assets.table.name'),
      accessor: (asset: BackendAsset) => (
        <Text fontSize="sm" fontWeight="medium" noOfLines={1}>
          {asset.name}
        </Text>
      )
    },
    {
      header: t('assets.table.category'),
      accessor: (asset: BackendAsset) => (
        <Text fontSize="sm" noOfLines={1}>
          {asset.category}
        </Text>
      )
    },
    {
      header: t('assets.table.purchasePrice'),
      accessor: (asset: BackendAsset) => (
        <Text fontSize="sm" fontWeight="medium" whiteSpace="nowrap">
          {formatCurrency(asset.purchase_price)}
        </Text>
      )
    },
    {
      header: t('assets.table.bookValue'),
      accessor: (asset: BackendAsset) => (
        <Text fontSize="sm" fontWeight="medium" whiteSpace="nowrap">
          {formatCurrency(calculateBookValue(asset))}
        </Text>
      )
    },
    {
      header: t('assets.table.status'),
      accessor: (asset: BackendAsset) => (
        <Badge colorScheme={getStatusColor(asset.status)} variant="subtle" size="sm">
          {asset.status}
        </Badge>
      )
    },
    {
      header: t('assets.table.location'),
      accessor: (asset: BackendAsset) => (
        <VStack align="start" spacing={1} maxW="180px">
          <Text noOfLines={1} fontSize="xs">
            {asset.location || t('assets.table.noLocation')}
          </Text>
          {asset.coordinates && (
            <HStack spacing={1}>
              <Text fontSize="xs" color="gray.500" fontFamily="mono" noOfLines={1}>
                {asset.coordinates}
              </Text>
              <Button
                size="xs"
                variant="ghost"
                colorScheme="blue"
                onClick={() => assetService.openInMaps(asset.coordinates!)}
                title={t('assets.details.viewOnMap')}
                minW="auto"
                p={1}
              >
                <FiMapPin size={10} />
              </Button>
            </HStack>
          )}
        </VStack>
      )
    },
  ];

  // Handle location picker
  const handleLocationPick = (locationData: { name: string; description: string; address: string; coordinates: string }) => {
    // Update coordinates field
    handleInputChange('coordinates', locationData.coordinates);

    // Update location field with comprehensive info
    let locationText = locationData.name;
    if (locationData.description) {
      locationText += ` - ${locationData.description}`;
    }
    if (locationData.address) {
      locationText += ` (${locationData.address})`;
    }
    handleInputChange('location', locationText);
  };

  // Action buttons for each row
  const renderActions = (asset: BackendAsset) => (
    <>
      <Button
        size="xs"
        variant="outline"
        leftIcon={<FiEye />}
        onClick={() => handleViewDetails(asset)}
        colorScheme="blue"
        minW="auto"
        px={2}
      >
        {t('common.view')}
      </Button>
      <Button
        size="xs"
        variant="outline"
        leftIcon={<FiEdit />}
        onClick={() => handleEdit(asset)}
        minW="auto"
        px={2}
      >
        {t('common.edit')}
      </Button>
      <Button
        size="xs"
        colorScheme="red"
        variant="outline"
        leftIcon={<FiTrash2 />}
        onClick={() => handleDelete(asset.id)}
        minW="auto"
        px={2}
      >
        {t('common.delete')}
      </Button>
      <Input
        type="file"
        accept="image/*"
        onChange={(e) => handleFileChange(e, asset.id)}
        style={{ display: 'none' }}
        id={`file-upload-${asset.id}`}
      />
      <Button
        size="xs"
        variant="outline"
        leftIcon={<FiUpload />}
        as="label"
        htmlFor={`file-upload-${asset.id}`}
        cursor="pointer"
        minW="auto"
        px={2}
        whiteSpace="nowrap"
      >
        {asset.image_path ? t('common.update') : t('common.buttons.upload')}
      </Button>
    </>
  );

  // Handle view asset details
  const handleViewDetails = async (asset: BackendAsset) => {
    setRecalcResult(null);
    setRecalcDate('');
    try {
      const response = await assetService.getAsset(asset.id);
      setDetailAsset(response.data);
      onDetailOpen();
    } catch (error: any) {
      console.error('Error fetching asset details:', error);
      toast({
        title: t('common.error'),
        description: t('assets.messages.detailsError'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    }
  };


  // Recalculate depreciation as of a date for selected detail asset
  const handleRecalculate = async () => {
    if (!detailAsset) return;
    try {
      setIsRecalcLoading(true);
      const res = await assetService.calculateDepreciation(detailAsset.id, recalcDate || undefined);
      setRecalcResult({
        current_book_value: res.data.current_book_value,
        accumulated_depreciation: res.data.accumulated_depreciation,
        depreciation_method: res.data.depreciation_method,
      });
      toast({ title: t('assets.messages.recalculated'), status: 'success', duration: 2000, isClosable: true });
    } catch (e) {
      toast({ title: t('assets.messages.recalculateFailed'), status: 'error', duration: 3000, isClosable: true });
    } finally {
      setIsRecalcLoading(false);
    }
  };

  // Handle export assets
  const handleExport = () => {
    if (assets.length === 0) {
      toast({
        title: t('assets.messages.exportNoData'),
        description: t('assets.messages.exportNoDataDesc'),
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    assetService.exportToCSV(assets);
    toast({
      title: t('assets.messages.exportStarted'),
      description: t('assets.messages.exportStartedDesc'),
      status: 'success',
      duration: 3000,
      isClosable: true,
    });
  };

  // Handle file change for image upload
  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>, assetId: number) => {
    if (e.target.files && e.target.files[0]) {
      const file = e.target.files[0];
      const asset = assets.find(a => a.id === assetId);

      if (asset && asset.image_path) {
        // Asset already has an image, show confirmation
        setPendingUpload({ assetId, file });
        onAlertOpen();
      } else {
        // No existing image, upload directly
        handleUpload(assetId, file);
      }
    }
  };

  // Confirm image update
  const confirmImageUpdate = () => {
    if (pendingUpload) {
      handleUpload(pendingUpload.assetId, pendingUpload.file);
      setPendingUpload(null);
    }
    onAlertClose();
  };

  // Handle direct upload
  const handleUpload = async (assetId: number, file: File) => {
    try {
      const response = await assetService.uploadAssetImage(assetId, file);

      // Update the asset in the list with the new image path
      setAssets(prevAssets =>
        prevAssets.map(asset =>
          asset.id === assetId
            ? { ...asset, image_path: response.path }
            : asset
        )
      );

      toast({
        title: t('assets.messages.imageUploadSuccess'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

      // Reset file input
      const fileInput = document.getElementById(`file-upload-${assetId}`) as HTMLInputElement;
      if (fileInput) {
        fileInput.value = '';
      }
    } catch (error) {
      toast({
        title: t('assets.messages.imageUploadFailed'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    }
  };

  // Handle image upload from component
  const handleImageUpload = async (updatedAsset: BackendAsset) => {
    // Update the selectedAsset with new image path
    setSelectedAsset(updatedAsset);

    // Update the assets list with the updated asset
    setAssets(prevAssets =>
      prevAssets.map(asset =>
        asset.id === updatedAsset.id ? updatedAsset : asset
      )
    );

    toast({
      title: t('assets.messages.imageUpdated'),
      description: t('assets.messages.imageUpdatedDesc'),
      status: 'success',
      duration: 3000,
      isClosable: true,
    });
  };

  // Category management functions
  const handleOpenCategoryModal = () => {
    setIsCategoryModalOpen(true);
    setNewCategoryName('');
    setNewCategoryCode('');
    setEditingCategoryIndex(null);
  };

  const handleCloseCategoryModal = () => {
    setIsCategoryModalOpen(false);
    setNewCategoryName('');
    setNewCategoryCode('');
    setEditingCategoryIndex(null);
  };

  const handleAddCategory = async () => {
    if (!newCategoryName.trim()) {
      toast({
        title: t('common.error'),
        description: t('assets.messages.categoryNameEmpty'),
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    if (!newCategoryCode.trim()) {
      toast({
        title: t('common.error'),
        description: t('assets.messages.categoryCodeEmpty'),
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    const code = newCategoryCode.trim().toUpperCase().replace(/[^A-Z0-9-]/g, '').substring(0, 10);
    if (code.length < 2) {
      toast({
        title: t('common.error'),
        description: t('assets.messages.categoryCodeMinLength'),
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    if (customCategories.includes(newCategoryName.trim())) {
      toast({
        title: t('common.error'),
        description: t('assets.messages.categoryExists'),
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    try {
      // Call backend API to create asset category
      const categoryData = {
        code,
        name: newCategoryName.trim(),
        description: '',
        is_active: true
      };

      await assetService.createAssetCategory(categoryData as any);

      // Refresh categories from database to get the latest list
      await fetchCategories();
      setNewCategoryName('');
      setNewCategoryCode('');

      toast({
        title: t('common.buttons.create'),
        description: t('assets.messages.categoryAdded'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: any) {
      console.error('Error creating category:', error);
      toast({
        title: t('common.error'),
        description: error.response?.data?.error || t('assets.messages.categoryNameEmpty'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    }
  };

  // Helper to get code for a given category name (from DB or default mapping)
  const getCodeForCategory = (name: string): string => {
    const fromDb = dbCategories.find(c => c.name === name)?.code;
    if (fromDb) return fromDb;
    // Default mapping (same as backend static mapping)
    const map: Record<string, string> = {
      'Fixed Asset': 'FA',
      'Real Estate': 'RE',
      'Computer Equipment': 'CE',
      'Vehicle': 'VH',
      'Office Equipment': 'OE',
      'Furniture': 'FR',
      'IT Infrastructure': 'IT',
      'Machinery': 'MC',
      'Tanah': 'LAND'
    };
    return map[name] || '';
  };

  const handleEditCategory = (index: number) => {
    setEditingCategoryIndex(index);
    const name = customCategories[index];
    setNewCategoryName(name);
    setNewCategoryCode(getCodeForCategory(name));
  };

  const handleUpdateCategory = () => {
    if (!newCategoryName.trim()) {
      toast({
        title: t('common.error'),
        description: t('assets.messages.categoryNameEmpty'),
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    if (editingCategoryIndex !== null) {
      const updatedCategories = [...customCategories];
      updatedCategories[editingCategoryIndex] = newCategoryName.trim();
      setCustomCategories(updatedCategories);
      setEditingCategoryIndex(null);
      setNewCategoryName('');
      toast({
        title: t('common.update'),
        description: t('assets.messages.categoryUpdated'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    }
  };

  const handleDeleteCategory = (index: number) => {
    const categoryToDelete = customCategories[index];

    // Check if category is being used by any assets
    const isUsed = assets.some(asset => asset.category === categoryToDelete);
    if (isUsed) {
      toast({
        title: t('common.error'),
        description: t('assets.messages.categoryInUse'),
        status: 'warning',
        duration: 5000,
        isClosable: true,
      });
      return;
    }

    if (window.confirm(`${t('assets.categories.confirmDelete')} "${categoryToDelete}"?`)) {
      const updatedCategories = customCategories.filter((_, i) => i !== index);
      setCustomCategories(updatedCategories);
      toast({
        title: t('common.delete'),
        description: t('assets.messages.categoryDeleted'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    }
  };

  const cancelEdit = () => {
    setEditingCategoryIndex(null);
    setNewCategoryName('');
  };



  // ... existing imports

  // ...

  return (
    <ProtectedModule module="assets">
      <CompanyTypeGuard>
        <SimpleLayout allowedRoles={['admin', 'finance', 'inventory_manager', 'director', 'employee']}>
          <Box>
            <Flex justify="space-between" align="center" mb={6}>
              <Box>
                <Heading size="lg" color={headingColor}>{t('assets.assetMaster')}</Heading>
                <Text color={mutedTextColor} mt={1}>
                  {t('assets.manageDescription')}
                </Text>
              </Box>
              <Flex gap={3}>
                <Button
                  variant="outline"
                  leftIcon={<FiDownload />}
                  onClick={handleExport}
                  isDisabled={assets.length === 0}
                >
                  {t('assets.export')}
                </Button>
                <Button
                  variant="outline"
                  leftIcon={<FiSettings />}
                  onClick={handleOpenCategoryModal}
                  colorScheme="gray"
                >
                  {t('assets.manageCategories')}
                </Button>
                <Button
                  colorScheme="blue"
                  leftIcon={<FiPlus />}
                  onClick={handleCreate}
                >
                  {t('assets.addAsset')}
                </Button>
              </Flex>
            </Flex>

            {error && (
              <Alert status="error" mb={4}>
                <AlertIcon />
                <AlertTitle mr={2}>Error!</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            <AssetSummaryComponent
              summary={summary}
              isLoading={isLoadingSummary}
            />

            {/* Filters */}
            <Box mb={4}>
              <HStack spacing={3} align="center" wrap="wrap">
                <Input
                  placeholder={t('assets.searchAssets')}
                  value={searchTerm}
                  onChange={(e) => { setSearchTerm(e.target.value); setCurrentPage(1); }}
                  maxW="280px"
                  bg={inputBg}
                  borderColor={inputBorderColor}
                  color={textColor}
                  _hover={{ borderColor: inputBorderColor }}
                  _focusVisible={fieldFocusStyles}
                  _placeholder={{ color: mutedTextColor }}
                />
                <Select
                  placeholder={t('assets.filters.allCategories')}
                  value={filterCategory}
                  onChange={(e) => { setFilterCategory(e.target.value); setCurrentPage(1); }}
                  maxW="240px"
                  bg={inputBg}
                  borderColor={inputBorderColor}
                  color={textColor}
                  _hover={{ borderColor: inputBorderColor }}
                  _focusVisible={fieldFocusStyles}
                >
                  {Array.from(new Set(assets.map(a => a.category).filter(Boolean))).map(cat => (
                    <option key={cat} value={cat}>{cat}</option>
                  ))}
                </Select>
                <Select
                  placeholder={t('assets.filters.allStatus')}
                  value={filterStatus}
                  onChange={(e) => { setFilterStatus(e.target.value); setCurrentPage(1); }}
                  maxW="200px"
                  bg={inputBg}
                  borderColor={inputBorderColor}
                  color={textColor}
                  _hover={{ borderColor: inputBorderColor }}
                  _focusVisible={fieldFocusStyles}
                >
                  <option value="PENDING">⏳ Pending</option>
                  <option value="ACTIVE">{t('assets.form.statusActive').toUpperCase()}</option>
                  <option value="INACTIVE">{t('assets.form.statusInactive').toUpperCase()}</option>
                  <option value="SOLD">{t('assets.form.statusSold').toUpperCase()}</option>
                </Select>
                <HStack spacing={2}>
                  <Text fontSize="sm" color={mutedTextColor}>{t('assets.filters.rows')}:</Text>
                  <Select value={pageSize} onChange={(e) => { setPageSize(parseInt(e.target.value) || 10); setCurrentPage(1); }} maxW="80px" bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles}>
                    <option value={5}>5</option>
                    <option value={10}>10</option>
                    <option value={20}>20</option>
                    <option value={50}>50</option>
                  </Select>
                </HStack>
              </HStack>
            </Box>

            {/* Filter & paginate data */}
            {(() => {
              const filtered = assets.filter(a => {
                const matchSearch = !searchTerm ||
                  a.code?.toLowerCase().includes(searchTerm.toLowerCase()) ||
                  a.name?.toLowerCase().includes(searchTerm.toLowerCase());
                const matchCat = !filterCategory || a.category === filterCategory;
                const matchStatus = !filterStatus || a.status === filterStatus;
                return matchSearch && matchCat && matchStatus;
              });
              const total = filtered.length;
              const totalPages = Math.max(1, Math.ceil(total / pageSize));
              const page = Math.min(currentPage, totalPages);
              const start = (page - 1) * pageSize;
              const pageData = filtered.slice(start, start + pageSize);

              return (
                <>
                  <Table<BackendAsset>
                    columns={columns}
                    data={pageData}
                    keyField="id"
                    title={`Assets (${total})`}
                    actions={renderActions}
                    isLoading={isLoading}
                    emptyMessage={t('assets.noAssets')}
                  />
                  {/* Pagination Controls */}
                  <HStack justify="space-between" mt={3}>
                    <Text fontSize="sm" color={mutedTextColor}>{t('assets.pagination.showing')} {Math.min(total, start + 1)}–{Math.min(total, start + pageSize)} {t('assets.pagination.of')} {total}</Text>
                    <HStack>
                      <Button size="sm" onClick={() => setCurrentPage(1)} isDisabled={page <= 1}>{t('assets.pagination.first')}</Button>
                      <Button size="sm" onClick={() => setCurrentPage(p => Math.max(1, p - 1))} isDisabled={page <= 1}>{t('assets.pagination.prev')}</Button>
                      <Text fontSize="sm" color={textColor}>{t('assets.pagination.page')} {page} / {totalPages}</Text>
                      <Button size="sm" onClick={() => setCurrentPage(p => Math.min(totalPages, p + 1))} isDisabled={page >= totalPages}>{t('assets.pagination.next')}</Button>
                      <Button size="sm" onClick={() => setCurrentPage(totalPages)} isDisabled={page >= totalPages}>{t('assets.pagination.last')}</Button>
                    </HStack>
                  </HStack>
                </>
              );
            })()}

            <Modal
              isOpen={isModalOpen}
              onClose={handleCloseModal}
              size="6xl"
            >
              <ModalOverlay />
              <ModalContent bg={modalBg} borderWidth="1px" borderColor={modalBorderColor}>
                <form onSubmit={handleSubmit}>
                  <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={modalBorderColor}>
                    {selectedAsset?.id ? t('assets.editAsset') : t('assets.addAsset')}
                  </ModalHeader>
                  <ModalCloseButton color={textColor} />

                  <ModalBody pb={6}>
                    {/* Information Banner */}
                    <Alert status="info" borderRadius="md" bg={infoAlertBg} border="1px solid" borderColor={infoAlertBorder}>
                      <AlertIcon color="blue.500" />
                      <Box>
                        <AlertTitle color={infoAlertTitleColor} fontSize="sm" fontWeight="bold">
                          📝 {t('assets.form.manualAssetEntry')}
                        </AlertTitle>
                        <AlertDescription color={infoAlertTextColor} fontSize="xs" mt={1}>
                          {t('assets.form.manualAssetEntryDescUpdated')}
                        </AlertDescription>
                      </Box>
                    </Alert>

                    <VStack spacing={6}>
                      {/* Basic Information Section */}
                      <Box w="full">
                        <Text fontSize="md" fontWeight="semibold" color={textColor} mb={4}>
                          📋 {t('assets.form.basicInfo')}
                        </Text>
                        <VStack spacing={4}>
                          <HStack w="full" spacing={4}>
                            <FormControl isRequired isInvalid={hasFieldError(validationErrors, 'name')}>
                              <FormLabel fontSize="sm" fontWeight="medium">{t('assets.assetName')}</FormLabel>
                              <Input
                                value={formData.name || ''}
                                onChange={(e) => handleInputChange('name', e.target.value)}
                                placeholder={t('assets.form.enterAssetName')}
                                size="md"
                                {...themedFieldStyles}
                              />
                              {getFieldError(validationErrors, 'name') && (
                                <Text fontSize="xs" color="red.500" mt={1}>
                                  {getFieldError(validationErrors, 'name')}
                                </Text>
                              )}
                            </FormControl>

                            <FormControl isRequired isInvalid={hasFieldError(validationErrors, 'category')}>
                              <FormLabel fontSize="sm" fontWeight="medium">{t('assets.category')}</FormLabel>
                              <Select
                                value={formData.category || ''}
                                onChange={(e) => handleInputChange('category', e.target.value)}
                                placeholder={t('assets.form.selectCategory')}
                                size="md"
                                {...themedFieldStyles}
                              >
                                {customCategories.map((category) => (
                                  <option key={category} value={category}>
                                    {category}
                                  </option>
                                ))}
                              </Select>
                              {getFieldError(validationErrors, 'category') && (
                                <Text fontSize="xs" color="red.500" mt={1}>
                                  {getFieldError(validationErrors, 'category')}
                                </Text>
                              )}
                            </FormControl>
                          </HStack>

                          {formData.entryType === 'opening' && (
                            <FormControl isInvalid={hasFieldError(validationErrors, 'openingAccumulatedDepreciation')}>
                              <FormLabel fontSize="sm" fontWeight="medium">Opening Accumulated Depreciation</FormLabel>
                              <CurrencyInput
                                value={formData.openingAccumulatedDepreciation || 0}
                                onChange={(value) => handleInputChange('openingAccumulatedDepreciation', value)}
                                placeholder="Isi akumulasi penyusutan saat saldo awal"
                                isInvalid={hasFieldError(validationErrors, 'openingAccumulatedDepreciation')}
                                errorMessage={getFieldError(validationErrors, 'openingAccumulatedDepreciation')}
                                size="md"
                                min={0}
                                showLabel={false}
                              />
                              <Text fontSize="xs" color={helperTextColor} mt={1}>
                                Nilai ini akan dijurnal sebagai kredit akumulasi penyusutan agar saldo awal aset masuk dengan nilai buku yang benar.
                              </Text>
                            </FormControl>
                          )}

                          <HStack w="full" spacing={4}>
                            <FormControl>
                              <FormLabel fontSize="sm" fontWeight="medium">{t('assets.form.serialNumber')}</FormLabel>
                              <Input
                                value={formData.serialNumber || ''}
                                onChange={(e) => handleInputChange('serialNumber', e.target.value)}
                                placeholder={t('assets.form.enterSerialNumber')}
                                size="md"
                                {...themedFieldStyles}
                              />
                            </FormControl>

                            <FormControl>
                              <FormLabel fontSize="sm" fontWeight="medium">{t('assets.form.condition')}</FormLabel>
                              <Select
                                value={formData.condition || 'Good'}
                                onChange={(e) => handleInputChange('condition', e.target.value)}
                                size="md"
                                {...themedFieldStyles}
                              >
                                <option value="Excellent">{t('assets.form.conditionExcellent')}</option>
                                <option value="Good">{t('assets.form.conditionGood')}</option>
                                <option value="Fair">{t('assets.form.conditionFair')}</option>
                                <option value="Poor">{t('assets.form.conditionPoor')}</option>
                              </Select>
                            </FormControl>
                          </HStack>
                        </VStack>
                      </Box>

                      {/* Financial Information Section */}
                      <Box w="full">
                        <Text fontSize="md" fontWeight="semibold" color={textColor} mb={4}>
                          💰 {t('assets.form.financialInfo')}
                        </Text>
                        <VStack spacing={4}>
                          <HStack w="full" spacing={4}>
                            <FormControl isRequired isInvalid={hasFieldError(validationErrors, 'purchaseDate')}>
                              <FormLabel fontSize="sm" fontWeight="medium">{t('assets.purchaseDate')}</FormLabel>
                              <Input
                                type="date"
                                value={formData.purchaseDate || ''}
                                onChange={(e) => handleInputChange('purchaseDate', e.target.value)}
                                size="md"
                                {...themedFieldStyles}
                              />
                              {getFieldError(validationErrors, 'purchaseDate') && (
                                <Text fontSize="xs" color="red.500" mt={1}>
                                  {getFieldError(validationErrors, 'purchaseDate')}
                                </Text>
                              )}
                            </FormControl>

                            <CurrencyInput
                              value={formData.purchasePrice || 0}
                              onChange={(value) => handleInputChange('purchasePrice', value)}
                              label={t('assets.purchasePrice')}
                              placeholder="Contoh: Rp 100.000.000"
                              isRequired={true}
                              isInvalid={hasFieldError(validationErrors, 'purchasePrice')}
                              errorMessage={getFieldError(validationErrors, 'purchasePrice')}
                              size="md"
                              min={1}
                            />
                          </HStack>

                          <HStack w="full" spacing={4}>
                            <FormControl isInvalid={hasFieldError(validationErrors, 'salvageValue')}>
                              <HStack spacing={2} align="center">
                                <FormLabel fontSize="sm" fontWeight="medium" mb={0}>{t('assets.form.salvageValue')}</FormLabel>
                                <Tooltip
                                  label={
                                    <Box>
                                      <Text fontWeight="semibold" mb={1}>💡 {t('assets.form.salvageValue')}</Text>
                                      <Text fontSize="xs" lineHeight="1.4">
                                        {t('assets.form.salvageValueTooltip')}
                                      </Text>
                                      <Text fontSize="xs" lineHeight="1.4" mt={1}>
                                        {t('assets.form.salvageValueExample')}
                                      </Text>
                                      <Text fontSize="xs" lineHeight="1.4" mt={1} fontWeight="medium">
                                        {t('assets.form.salvageValueNote')}
                                      </Text>
                                    </Box>
                                  }
                                  hasArrow
                                  placement="top"
                                  bg={tooltipBg}
                                  color="white"
                                  borderRadius="md"
                                  p={3}
                                  maxW="280px"
                                >
                                  <span><Icon as={FiInfo} color="blue.500" boxSize={4} /></span>
                                </Tooltip>
                              </HStack>
                              <CurrencyInput
                                value={formData.salvageValue || 0}
                                onChange={(value) => handleInputChange('salvageValue', value)}
                                placeholder="Contoh: Rp 5.000.000"
                                isInvalid={hasFieldError(validationErrors, 'salvageValue')}
                                errorMessage={getFieldError(validationErrors, 'salvageValue')}
                                size="md"
                                min={0}
                                showLabel={false}
                              />
                            </FormControl>

                            <FormControl isInvalid={hasFieldError(validationErrors, 'usefulLife')}>
                              <FormLabel fontSize="sm" fontWeight="medium">{t('assets.form.usefulLife')}</FormLabel>
                              <NumberInput
                                value={formData.usefulLife || 1}
                                onChange={(valueString) => handleInputChange('usefulLife', parseInt(valueString) || 1)}
                                min={1}
                                max={100}
                                size="md"
                              >
                                <NumberInputField {...themedFieldStyles} />
                                <NumberInputStepper>
                                  <NumberIncrementStepper />
                                  <NumberDecrementStepper />
                                </NumberInputStepper>
                              </NumberInput>
                              {getFieldError(validationErrors, 'usefulLife') && (
                                <Text fontSize="xs" color="red.500" mt={1}>
                                  {getFieldError(validationErrors, 'usefulLife')}
                                </Text>
                              )}
                            </FormControl>
                          </HStack>

                          <HStack w="full" spacing={4}>
                            <FormControl>
                              <HStack spacing={2} align="center">
                                <FormLabel fontSize="sm" fontWeight="medium" mb={0}>{t('assets.form.depreciationMethod')}</FormLabel>
                                <Tooltip
                                  label={
                                    <Box>
                                      <Text fontWeight="semibold" mb={1}>💡 {t('assets.form.depreciationMethod')}</Text>
                                      <Text fontSize="xs">{t('assets.form.depreciationMethodTooltip')}</Text>
                                    </Box>
                                  }
                                  hasArrow
                                  placement="top"
                                  bg={tooltipBg}
                                  color="white"
                                  borderRadius="md"
                                  p={3}
                                >
                                  <span><Icon as={FiInfo} color="blue.500" boxSize={4} /></span>
                                </Tooltip>
                              </HStack>
                              <Select
                                value={formData.depreciationMethod || 'STRAIGHT_LINE'}
                                onChange={(e) => handleInputChange('depreciationMethod', e.target.value as 'STRAIGHT_LINE' | 'DECLINING_BALANCE')}
                                size="md"
                                {...themedFieldStyles}
                              >
                                <option value="STRAIGHT_LINE">{DEPRECIATION_METHOD_LABELS.STRAIGHT_LINE}</option>
                                <option value="DECLINING_BALANCE">{DEPRECIATION_METHOD_LABELS.DECLINING_BALANCE}</option>
                              </Select>
                            </FormControl>

                            <FormControl>
                              <FormLabel fontSize="sm" fontWeight="medium">{t('assets.form.assetStatus')}</FormLabel>
                              <Select
                                value={formData.status || 'ACTIVE'}
                                onChange={(e) => handleInputChange('status', e.target.value)}
                                size="md"
                                {...themedFieldStyles}
                              >
                                <option value="PENDING">⏳ Pending</option>
                                <option value="ACTIVE">🟢 {t('assets.form.statusActive')}</option>
                                <option value="INACTIVE">⚪ {t('assets.form.statusInactive')}</option>
                                <option value="SOLD">🔴 {t('assets.form.statusSold')}</option>
                              </Select>
                            </FormControl>
                          </HStack>

                          {/* Asset and Depreciation Account Selection */}
                          <HStack w="full" spacing={4}>
                            <FormControl>
                              <FormLabel fontSize="sm" fontWeight="medium">🏢 {t('assets.form.fixedAssetAccount')}</FormLabel>
                              <Select
                                value={formData.assetAccountId ? String(formData.assetAccountId) : ''}
                                onChange={(e) => handleInputChange('assetAccountId', e.target.value ? parseInt(e.target.value) : undefined)}
                                placeholder={t('assets.form.chooseFixedAssetAccount')}
                                size="md"
                                isDisabled={isLoadingAccounts || (selectedAsset?.notes?.includes('Auto-created from Purchase') ?? false)}
                                {...themedFieldStyles}
                              >
                                {fixedAssetAccounts.map((account) => (
                                  <option key={account.id} value={String(account.id)}>
                                    {account.code} - {account.name} ({formatCurrency(account.balance)})
                                  </option>
                                ))}
                              </Select>
                              <Text fontSize="xs" color={helperTextColor} mt={1}>
                                💡 {selectedAsset?.notes?.includes('Auto-created from Purchase') ? t('assets.form.fixedAssetAccountAutoNote') : t('assets.form.fixedAssetAccountNote')}
                              </Text>
                            </FormControl>

                            <FormControl isInvalid={hasFieldError(validationErrors, 'depreciationAccountId')}>
                              <FormLabel fontSize="sm" fontWeight="medium">📉 {t('assets.form.depreciationExpenseAccount')}</FormLabel>
                              <Select
                                value={formData.depreciationAccountId ? String(formData.depreciationAccountId) : ''}
                                onChange={(e) => handleInputChange('depreciationAccountId', e.target.value ? parseInt(e.target.value) : undefined)}
                                placeholder={t('assets.form.chooseDepreciationAccount')}
                                size="md"
                                isDisabled={isLoadingAccounts}
                                {...themedFieldStyles}
                              >
                                {/* Fallback option: if saved account isn't in the dropdown list, render it from asset data */}
                                {formData.depreciationAccountId &&
                                  selectedAsset?.depreciation_account &&
                                  !depreciationAccounts.some((a: any) => a.id === formData.depreciationAccountId) && (
                                  <option key={`fallback-dep-${formData.depreciationAccountId}`} value={String(formData.depreciationAccountId)}>
                                    {selectedAsset.depreciation_account.code} - {selectedAsset.depreciation_account.name} ({formatCurrency(selectedAsset.depreciation_account.balance || 0)})
                                  </option>
                                )}
                                {depreciationAccounts.map((account) => (
                                  <option key={account.id} value={String(account.id)}>
                                    {account.code} - {account.name} ({formatCurrency(account.balance)})
                                  </option>
                                ))}
                              </Select>
                              {hasFieldError(validationErrors, 'depreciationAccountId') ? (
                                <Text fontSize="xs" color="red.500" mt={1}>
                                  {getFieldError(validationErrors, 'depreciationAccountId')}
                                </Text>
                              ) : (
                                <Text fontSize="xs" color={helperTextColor} mt={1}>
                                  💡 {t('assets.form.depreciationAccountNote')}
                                </Text>
                              )}
                            </FormControl>
                          </HStack>

                          {/* Accumulated Depreciation Account */}
                          <HStack w="full" spacing={4}>
                            <FormControl isInvalid={hasFieldError(validationErrors, 'accumulatedDepreciationAccountId')}>
                              <FormLabel fontSize="sm" fontWeight="medium">📊 {t('assets.form.accumulatedDepreciationAccount')}</FormLabel>
                              <Select
                                value={formData.accumulatedDepreciationAccountId ? String(formData.accumulatedDepreciationAccountId) : ''}
                                onChange={(e) => handleInputChange('accumulatedDepreciationAccountId', e.target.value ? parseInt(e.target.value) : undefined)}
                                placeholder={t('assets.form.chooseAccumulatedDepreciationAccount')}
                                size="md"
                                isDisabled={isLoadingAccounts}
                                {...themedFieldStyles}
                              >
                                {accumulatedDepreciationAccounts.map((account) => (
                                  <option key={account.id} value={String(account.id)}>
                                    {account.code} - {account.name} ({formatCurrency(account.balance)})
                                  </option>
                                ))}
                              </Select>
                              {hasFieldError(validationErrors, 'accumulatedDepreciationAccountId') ? (
                                <Text fontSize="xs" color="red.500" mt={1}>
                                  {getFieldError(validationErrors, 'accumulatedDepreciationAccountId')}
                                </Text>
                              ) : (
                                <Text fontSize="xs" color={helperTextColor} mt={1}>
                                  💡 {t('assets.form.accumulatedDepreciationAccountNote')}
                                </Text>
                              )}
                            </FormControl>
                          </HStack>
                        </VStack>
                      </Box>

                      {/* Entry Type & Payment Method Section - only for create mode */}
                      {!selectedAsset?.id && (
                      <Box w="full">
                        <Text fontSize="md" fontWeight="semibold" color={textColor} mb={4}>
                          💳 {t('assets.form.entryType')}
                        </Text>
                        <VStack spacing={4}>
                          {/* Entry Type Selection */}
                          <FormControl>
                            <FormLabel fontSize="sm" fontWeight="medium">{t('assets.form.entryType')}</FormLabel>
                            <HStack spacing={4} w="full">
                              <Box
                                flex={1}
                                p={3}
                                borderWidth="2px"
                                borderColor={formData.entryType === 'purchase' ? 'blue.500' : entryCardBorderColor}
                                borderRadius="md"
                                cursor="pointer"
                                bg={formData.entryType === 'purchase' ? purchaseEntryBg : entryCardBg}
                                onClick={() => {
                                  handleInputChange('entryType', 'purchase');
                                  handleInputChange('openingAccumulatedDepreciation', 0);
                                  handleInputChange('paymentMethod', 'CASH');
                                }}
                                _hover={{ borderColor: 'blue.300' }}
                              >
                                <Text fontWeight="semibold" fontSize="sm">🛒 {t('assets.form.entryTypePurchase')}</Text>
                                <Text fontSize="xs" color={formData.entryType === 'purchase' ? purchaseEntryTextColor : helperTextColor}>{t('assets.form.entryTypePurchaseDesc')}</Text>
                              </Box>
                              <Box
                                flex={1}
                                p={3}
                                borderWidth="2px"
                                borderColor={formData.entryType === 'opening' ? 'green.500' : entryCardBorderColor}
                                borderRadius="md"
                                cursor="pointer"
                                bg={formData.entryType === 'opening' ? openingEntryBg : entryCardBg}
                                onClick={() => {
                                  handleInputChange('entryType', 'opening');
                                  handleInputChange('paymentMethod', 'SALDO_AWAL');
                                  handleInputChange('paymentAccountId', undefined);
                                  handleInputChange('creditAccountId', undefined);
                                }}
                                _hover={{ borderColor: 'green.300' }}
                              >
                                <Text fontWeight="semibold" fontSize="sm">📦 {t('assets.form.entryTypeOpening')}</Text>
                                <Text fontSize="xs" color={formData.entryType === 'opening' ? openingEntryTextColor : helperTextColor}>{t('assets.form.entryTypeOpeningDesc')}</Text>
                              </Box>
                            </HStack>
                          </FormControl>

                          {/* Payment Method - only for Purchase */}
                          {formData.entryType === 'purchase' && (
                            <>
                              <FormControl>
                                <FormLabel fontSize="sm" fontWeight="medium">{t('assets.form.paymentMethod')}</FormLabel>
                                <Select
                                  value={formData.paymentMethod || 'CASH'}
                                  onChange={(e) => {
                                    handleInputChange('paymentMethod', e.target.value);
                                    handleInputChange('paymentAccountId', undefined);
                                    handleInputChange('creditAccountId', undefined);
                                  }}
                                  size="md"
                                  {...themedFieldStyles}
                                >
                                  <option value="CASH">{t('assets.form.paymentMethodCash')}</option>
                                  <option value="BANK">{t('assets.form.paymentMethodBank')}</option>
                                  <option value="CREDIT">{t('assets.form.paymentMethodCredit')}</option>
                                </Select>
                                <Text fontSize="xs" color={helperTextColor} mt={1}>
                                  💡 {t('assets.form.paymentMethodNote')}
                                </Text>
                              </FormControl>

                              {/* Cash/Bank Account Selection */}
                              {(formData.paymentMethod === 'CASH' || formData.paymentMethod === 'BANK') && (
                                <FormControl>
                                  <FormLabel fontSize="sm" fontWeight="medium">{t('assets.form.paymentAccount')}</FormLabel>
                                  <Select
                                    value={formData.paymentAccountId ? String(formData.paymentAccountId) : ''}
                                    onChange={(e) => handleInputChange('paymentAccountId', e.target.value ? parseInt(e.target.value) : undefined)}
                                    placeholder={t('assets.form.selectPaymentAccount')}
                                    size="md"
                                    isDisabled={isLoadingAccounts}
                                    {...themedFieldStyles}
                                  >
                                    {bankAccounts
                                      .filter((acc: any) => {
                                        const type = (acc.type || '').toUpperCase();
                                        if (formData.paymentMethod === 'CASH') return type === 'CASH';
                                        if (formData.paymentMethod === 'BANK') return type === 'BANK';
                                        return true;
                                      })
                                      .map((account: any) => (
                                        <option key={account.id} value={String(account.account_id || account.id)}>
                                          {account.display_name || `${account.code || ''} - ${account.name}`} ({account.balance_formatted || ''})
                                        </option>
                                      ))}
                                  </Select>
                                  <Alert status="info" mt={2} borderRadius="md" py={2}>
                                    <AlertIcon boxSize="14px" />
                                    <AlertDescription fontSize="xs">
                                      {t('assets.form.purchaseJournalInfo')} {formData.paymentMethod === 'CASH' ? 'Kas' : 'Bank'}
                                    </AlertDescription>
                                  </Alert>
                                </FormControl>
                              )}

                              {/* Credit/Liability Account Selection */}
                              {formData.paymentMethod === 'CREDIT' && (
                                <FormControl>
                                  <FormLabel fontSize="sm" fontWeight="medium">{t('assets.form.creditAccount')}</FormLabel>
                                  <Select
                                    value={formData.creditAccountId ? String(formData.creditAccountId) : ''}
                                    onChange={(e) => handleInputChange('creditAccountId', e.target.value ? parseInt(e.target.value) : undefined)}
                                    placeholder={t('assets.form.selectCreditAccount')}
                                    size="md"
                                    isDisabled={isLoadingAccounts}
                                    {...themedFieldStyles}
                                  >
                                    {(liabilityAccounts?.data || liabilityAccounts || []).filter((acc: any) => !acc.is_header && acc.is_active !== false).map((account: any) => (
                                      <option key={account.id} value={String(account.id)}>
                                        {account.code} - {account.name}
                                      </option>
                                    ))}
                                  </Select>
                                  <Alert status="info" mt={2} borderRadius="md" py={2}>
                                    <AlertIcon boxSize="14px" />
                                    <AlertDescription fontSize="xs">
                                      {t('assets.form.purchaseJournalInfo')} Hutang Usaha
                                    </AlertDescription>
                                  </Alert>
                                </FormControl>
                              )}
                            </>
                          )}

                          {/* Opening Balance Info */}
                          {formData.entryType === 'opening' && (
                            <Alert status="success" borderRadius="md" bg={openingAlertBg} border="1px solid" borderColor={openingAlertBorder}>
                              <AlertIcon />
                              <Box>
                                <Text fontWeight="semibold" fontSize="sm" color={openingAlertTitleColor}>{t('assets.form.openingJournalInfo')}</Text>
                                <Text fontSize="xs" color={openingAlertTextColor}>{t('assets.form.openingBalanceNote')}</Text>
                              </Box>
                            </Alert>
                          )}
                        </VStack>
                      </Box>
                      )}

                      {/* Location Information Section */}
                      <Box w="full">
                        <Text fontSize="md" fontWeight="semibold" color={textColor} mb={4}>
                          📍 {t('assets.form.locationInfo')}
                        </Text>
                        <VStack spacing={4}>
                          <FormControl>
                            <FormLabel fontSize="sm" fontWeight="medium">{t('assets.form.physicalLocation')}</FormLabel>
                            <Input
                              value={formData.location || ''}
                              onChange={(e) => handleInputChange('location', e.target.value)}
                              placeholder={t('assets.form.enterPhysicalLocation')}
                              size="md"
                              {...themedFieldStyles}
                            />
                            <Text fontSize="xs" color={helperTextColor} mt={1}>
                              💡 {t('assets.form.physicalLocationNote')}
                            </Text>
                          </FormControl>

                          <FormControl>
                            <FormLabel fontSize="sm" fontWeight="medium">{t('assets.form.gpsCoordinates')}</FormLabel>
                            <HStack spacing={3}>
                              <Input
                                value={formData.coordinates || ''}
                                onChange={(e) => handleInputChange('coordinates', e.target.value)}
                                placeholder={t('assets.form.selectOnMap')}
                                readOnly
                                flex={1}
                                size="md"
                                {...readonlyFieldStyles}
                              />
                              <Button
                                leftIcon={<FiMapPin />}
                                onClick={() => setIsMapPickerOpen(true)}
                                colorScheme="blue"
                                variant="outline"
                                size="md"
                                flexShrink={0}
                              >
                                {t('assets.form.selectOnMap')}
                              </Button>
                              {formData.coordinates && (
                                <Button
                                  leftIcon={<FiExternalLink />}
                                  onClick={() => assetService.openInMaps(formData.coordinates!)}
                                  colorScheme="green"
                                  variant="outline"
                                  size="md"
                                  flexShrink={0}
                                >
                                  {t('assets.form.viewOnMap')}
                                </Button>
                              )}
                            </HStack>
                            <Text fontSize="xs" color={helperTextColor} mt={1}>
                              🗺️ {t('assets.form.gpsNote')}
                            </Text>
                          </FormControl>
                        </VStack>
                      </Box>

                      {/* Asset Image Section */}
                      <Box w="full">
                        <Text fontSize="md" fontWeight="semibold" color={textColor} mb={4}>
                          📸 {t('assets.form.assetImage')}
                        </Text>
                        {selectedAsset && selectedAsset.id ? (
                          /* Edit Mode - Full upload functionality */
                          <AssetImageUpload
                            asset={selectedAsset}
                            onImageUpload={handleImageUpload}
                            size="lg"
                            showLabel={false}
                          />
                        ) : (
                          /* Create Mode - Inform user to save first */
                          <Box
                            p={6}
                            border="2px dashed"
                            borderColor={emptyStateBorder}
                            borderRadius="lg"
                            textAlign="center"
                            bg={emptyStateBg}
                          >
                            <VStack spacing={3}>
                              <Box
                                p={3}
                                bg={infoAlertBg}
                                borderRadius="full"
                                border="1px"
                                borderColor={infoAlertBorder}
                              >
                                <Icon as={FiEdit} boxSize={6} color="blue.500" />
                              </Box>
                              <Text fontSize="md" fontWeight="medium" color={textColor}>
                                {t('assets.form.saveAssetFirst')}
                              </Text>
                              <Text fontSize="sm" color={mutedTextColor} textAlign="center">
                                {t('assets.form.saveAssetFirstDesc')}
                              </Text>
                            </VStack>
                          </Box>
                        )}
                      </Box>

                      {/* Additional Notes Section */}
                      <Box w="full">
                        <Text fontSize="md" fontWeight="semibold" color={textColor} mb={4}>
                          📝 {t('assets.form.additionalInfo')}
                        </Text>
                        <FormControl>
                          <FormLabel fontSize="sm" fontWeight="medium">{t('assets.form.notes')}</FormLabel>
                          <Textarea
                            value={formData.notes || ''}
                            onChange={(e) => handleInputChange('notes', e.target.value)}
                            placeholder={t('assets.form.enterNotes')}
                            rows={4}
                            resize="vertical"
                            size="md"
                            {...themedFieldStyles}
                          />
                        </FormControl>
                      </Box>
                    </VStack>
                  </ModalBody>

                  <ModalFooter pb={6} bg={modalFooterBg} borderTopWidth="1px" borderColor={modalBorderColor}>
                    <HStack justify="flex-end" spacing={4}>
                      <Button
                        leftIcon={<FiX />}
                        onClick={handleCloseModal}
                        variant="outline"
                      >
                        {t('common.cancel')}
                      </Button>
                      <Button
                        leftIcon={selectedAsset?.id ? <FiEdit /> : <FiPlus />}
                        type="submit"
                        colorScheme="blue"
                        isLoading={isSubmitting}
                        loadingText={selectedAsset?.id ? t('common.updating') : t('common.creating')}
                      >
                        {selectedAsset?.id ? t('assets.updateAsset') : t('assets.createAsset')}
                      </Button>
                    </HStack>
                  </ModalFooter>
                </form>
              </ModalContent>
            </Modal>

            {/* Interactive Map Picker Modal */}
            <InteractiveMapPicker
              isOpen={isMapPickerOpen}
              onClose={() => setIsMapPickerOpen(false)}
              onLocationSelect={handleLocationPick}
              currentCoordinates={formData.coordinates}
              currentLocationData={{
                name: formData.location ? formData.location.split(' - ')[0] : '',
                description: '',
                address: '',
                coordinates: formData.coordinates || ''
              }}
              title={selectedAsset ? `${t('assets.form.selectOnMap')} - ${selectedAsset.name}` : t('assets.form.selectOnMap')}
            />

            {/* Image Update Confirmation Dialog */}
            <AlertDialog
              isOpen={isAlertOpen}
              leastDestructiveRef={React.useRef(null)}
              onClose={onAlertClose}
            >
              <AlertDialogOverlay>
                <AlertDialogContent>
                  <AlertDialogHeader fontSize="lg" fontWeight="bold">
                    {t('assets.imageUpload.title')}
                  </AlertDialogHeader>

                  <AlertDialogBody>
                    {t('assets.imageUpload.confirmReplace')}
                  </AlertDialogBody>

                  <AlertDialogFooter>
                    <Button onClick={onAlertClose}>
                      {t('common.cancel')}
                    </Button>
                    <Button colorScheme="blue" onClick={confirmImageUpdate} ml={3}>
                      {t('assets.imageUpload.updateImage')}
                    </Button>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialogOverlay>
            </AlertDialog>

            {/* Asset Details View Modal */}
            <Modal isOpen={isDetailOpen} onClose={onDetailClose} size="6xl">
              <ModalOverlay />
              <ModalContent bg={modalBg} borderWidth="1px" borderColor={modalBorderColor}>
                <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={modalBorderColor}>
                  <HStack spacing={3}>
                    <Icon as={FiEye} color="blue.500" />
                    <Text>{t('assets.assetDetails')}</Text>
                  </HStack>
                </ModalHeader>
                <ModalCloseButton color={textColor} />

                <ModalBody pb={6}>
                  {detailAsset && (
                    <VStack spacing={6} align="stretch">
                      {/* Asset Image Section */}
                      <Box textAlign="center">
                        {detailAsset.image_path ? (
                          <Image
                            src={getAssetImageUrl(detailAsset.image_path) || ''}
                            alt={detailAsset.name}
                            maxH="300px"
                            maxW="400px"
                            objectFit="cover"
                            borderRadius="lg"
                            boxShadow="lg"
                            mx="auto"
                            fallback={
                              <Box
                                w="300px"
                                h="200px"
                                bg={detailPlaceholderBg}
                                borderRadius="lg"
                                display="flex"
                                alignItems="center"
                                justifyContent="center"
                                mx="auto"
                              >
                                <Text color={mutedTextColor} fontSize="lg">
                                  {t('assets.details.failedToLoadImage')}
                                </Text>
                              </Box>
                            }
                          />
                        ) : (
                          <Box
                            w="300px"
                            h="200px"
                            bg={detailPlaceholderBg}
                            borderRadius="lg"
                            display="flex"
                            alignItems="center"
                            justifyContent="center"
                            mx="auto"
                          >
                            <Text color={mutedTextColor} fontSize="lg">
                              {t('assets.details.noImageAvailable')}
                            </Text>
                          </Box>
                        )}
                      </Box>

                      {/* Basic Information */}
                      <Box>
                        <Text fontSize="xl" fontWeight="bold" mb={4} color={textColor}>
                          📋 {t('assets.details.basicInfo')}
                        </Text>
                        <Grid templateColumns="repeat(2, 1fr)" gap={6}>
                          <GridItem>
                            <VStack align="start" spacing={4}>
                              <Box>
                                <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.assetCode')}</Text>
                                <Text fontSize="lg" fontWeight="semibold" color="blue.600">
                                  {detailAsset.code}
                                </Text>
                              </Box>
                              <Box>
                                <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.assetName')}</Text>
                                <Text fontSize="md" fontWeight="medium">
                                  {detailAsset.name}
                                </Text>
                              </Box>
                              <Box>
                                <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.category')}</Text>
                                <Badge colorScheme="purple" size="lg" px={3} py={1} fontSize="sm">
                                  {detailAsset.category}
                                </Badge>
                              </Box>
                            </VStack>
                          </GridItem>
                          <GridItem>
                            <VStack align="start" spacing={4}>
                              <Box>
                                <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.serialNumber')}</Text>
                                <Text fontSize="md">
                                  {detailAsset.serial_number || t('assets.details.notSpecified')}
                                </Text>
                              </Box>
                              <Box>
                                <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.condition')}</Text>
                                <Text fontSize="md">
                                  {detailAsset.condition || 'Good'}
                                </Text>
                              </Box>
                              <Box>
                                <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.status')}</Text>
                                <Badge colorScheme={getStatusColor(detailAsset.status)} size="lg" px={3} py={1}>
                                  {detailAsset.status === 'PENDING' ? '⏳ Pending' : 
                                   detailAsset.status === 'ACTIVE' ? '🟢 Aktif' : 
                                   detailAsset.status === 'SOLD' ? '🔴 Terjual' : '⚪ Tidak Aktif'}
                                </Badge>
                              </Box>
                            </VStack>
                          </GridItem>
                        </Grid>
                      </Box>

                      {/* Financial Information */}
                      <Box>
                        <Text fontSize="xl" fontWeight="bold" mb={4} color={textColor}>
                          💰 {t('assets.details.financialInfo')}
                        </Text>

                        {/* Recalculate as of date */}
                        <Box mb={4}>
                          <HStack spacing={3} align="center">
                            <Text fontSize="sm" color={mutedTextColor}>{t('assets.details.recalculateAsOf')}</Text>
                            <Input type="date" size="sm" value={recalcDate} onChange={(e) => setRecalcDate(e.target.value)} maxW="200px" bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles} />
                            <Button size="sm" colorScheme="blue" isLoading={isRecalcLoading} onClick={handleRecalculate}>{t('assets.details.recalculate')}</Button>
                          </HStack>
                          {recalcResult && (
                            <HStack spacing={6} mt={3}>
                              <Badge colorScheme="teal">{t('assets.details.currentBookValue')}: {formatCurrency(recalcResult.current_book_value)}</Badge>
                              <Badge colorScheme="orange">{t('assets.details.accumulatedDepreciation')}: {formatCurrency(recalcResult.accumulated_depreciation)}</Badge>
                              <Badge colorScheme="purple">{t('assets.details.depreciationMethod')}: {recalcResult.depreciation_method.replace('_', ' ')}</Badge>
                            </HStack>
                          )}
                        </Box>
                        <Grid templateColumns="repeat(2, 1fr)" gap={6}>
                          <GridItem>
                            <VStack align="start" spacing={4}>
                              <Box>
                                <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.purchaseDate')}</Text>
                                <Text fontSize="md">
                                  {new Date(detailAsset.purchase_date).toLocaleDateString('id-ID', {
                                    year: 'numeric',
                                    month: 'long',
                                    day: 'numeric'
                                  })}
                                </Text>
                              </Box>
                              <Box>
                                <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.purchasePrice')}</Text>
                                <Text fontSize="lg" fontWeight="semibold" color="green.600">
                                  {formatCurrency(detailAsset.purchase_price)}
                                </Text>
                              </Box>
                              <Box>
                                <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.salvageValue')}</Text>
                                <Text fontSize="md">
                                  {formatCurrency(detailAsset.salvage_value)}
                                </Text>
                              </Box>
                            </VStack>
                          </GridItem>
                          <GridItem>
                            <VStack align="start" spacing={4}>
                              <Box>
                                <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.usefulLife')}</Text>
                                <Text fontSize="md">
                                  {detailAsset.useful_life} {t('assets.details.years')}
                                </Text>
                              </Box>
                              <Box>
                                <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.accumulatedDepreciation')}</Text>
                                <Text fontSize="lg" fontWeight="semibold" color="orange.600">
                                  {formatCurrency(detailAsset.accumulated_depreciation)}
                                </Text>
                              </Box>
                              <Box>
                                <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.currentBookValue')}</Text>
                                <Text fontSize="lg" fontWeight="bold" color="blue.600">
                                  {formatCurrency(calculateBookValue(detailAsset))}
                                </Text>
                              </Box>
                            </VStack>
                          </GridItem>
                        </Grid>
                        <Box mt={4}>
                          <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.depreciationMethod')}</Text>
                          <Badge colorScheme="teal" size="lg" px={3} py={1}>
                            {DEPRECIATION_METHOD_LABELS[detailAsset.depreciation_method as keyof typeof DEPRECIATION_METHOD_LABELS]}
                          </Badge>
                        </Box>
                      </Box>

                      {/* Location Information */}
                      {(detailAsset.location || detailAsset.coordinates) && (
                        <Box>
                          <Text fontSize="xl" fontWeight="bold" mb={4} color={textColor}>
                            📍 {t('assets.details.locationInfo')}
                          </Text>
                          <VStack align="start" spacing={3}>
                            {detailAsset.location && (
                              <Box>
                                <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.physicalLocation')}</Text>
                                <Text fontSize="md">
                                  {detailAsset.location}
                                </Text>
                              </Box>
                            )}
                            {detailAsset.coordinates && (
                              <Box>
                                <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.gpsCoordinates')}</Text>
                                <HStack spacing={3}>
                                  <Text fontSize="md" fontFamily="mono">
                                    {detailAsset.coordinates}
                                  </Text>
                                  <Button
                                    size="sm"
                                    leftIcon={<FiExternalLink />}
                                    onClick={() => assetService.openInMaps(detailAsset.coordinates!)}
                                    colorScheme="blue"
                                    variant="outline"
                                  >
                                    {t('assets.details.viewOnMap')}
                                  </Button>
                                </HStack>
                              </Box>
                            )}
                          </VStack>
                        </Box>
                      )}

                      {/* Additional Notes */}
                      {detailAsset.notes && (
                        <Box>
                          <Text fontSize="xl" fontWeight="bold" mb={4} color={textColor}>
                            📝 {t('assets.details.notes')}
                          </Text>
                          <Box
                            p={4}
                            bg={panelBg}
                            borderRadius="lg"
                            border="1px solid"
                            borderColor={modalBorderColor}
                          >
                            <Text fontSize="md" whiteSpace="pre-wrap">
                              {detailAsset.notes}
                            </Text>
                          </Box>
                        </Box>
                      )}

                      {/* Timestamps */}
                      <Box>
                        <Text fontSize="lg" fontWeight="semibold" mb={3} color={mutedTextColor}>
                          📅 {t('assets.details.recordInfo')}
                        </Text>
                        <Grid templateColumns="repeat(2, 1fr)" gap={4}>
                          <GridItem>
                            <Box>
                              <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.createdAt')}</Text>
                              <Text fontSize="sm">
                                {new Date(detailAsset.created_at).toLocaleString('id-ID')}
                              </Text>
                            </Box>
                          </GridItem>
                          <GridItem>
                            <Box>
                              <Text fontSize="sm" color={mutedTextColor} fontWeight="medium">{t('assets.details.lastUpdated')}</Text>
                              <Text fontSize="sm">
                                {new Date(detailAsset.updated_at).toLocaleString('id-ID')}
                              </Text>
                            </Box>
                          </GridItem>
                        </Grid>
                      </Box>
                    </VStack>
                  )}
                </ModalBody>

                <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={modalBorderColor}>
                  <HStack spacing={3}>
                    <Button
                      leftIcon={<FiEdit />}
                      onClick={() => {
                        if (detailAsset) {
                          onDetailClose();
                          handleEdit(detailAsset);
                        }
                      }}
                      colorScheme="blue"
                      variant="outline"
                    >
                      {t('assets.editAsset')}
                    </Button>
                    <Button onClick={onDetailClose}>
                      {t('common.close')}
                    </Button>
                  </HStack>
                </ModalFooter>
              </ModalContent>
            </Modal>

            {/* Category Management Modal */}
            <Modal isOpen={isCategoryModalOpen} onClose={handleCloseCategoryModal} size="lg">
              <ModalOverlay />
              <ModalContent bg={modalBg} borderWidth="1px" borderColor={modalBorderColor}>
                <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={modalBorderColor}>
                  <HStack spacing={3}>
                    <Icon as={FiSettings} color={mutedTextColor} />
                    <Text>{t('assets.categories.title')}</Text>
                  </HStack>
                </ModalHeader>
                <ModalCloseButton color={textColor} />

                <ModalBody pb={6}>
                  <VStack spacing={6} align="stretch">
                    {/* Add New Category Section */}
                    <Box>
                      <Text fontSize="md" fontWeight="semibold" color={textColor} mb={4}>
                        ➕ {t('assets.categories.addNew')}
                      </Text>
                      <HStack spacing={3}>
                        <FormControl flex={1}>
                          <Input
                            value={newCategoryName}
                            onChange={(e) => setNewCategoryName(e.target.value)}
                            placeholder={t('assets.categories.enterCategoryName')}
                            bg={inputBg}
                            borderColor={inputBorderColor}
                            color={textColor}
                            _hover={{ borderColor: inputBorderColor }}
                            _focusVisible={fieldFocusStyles}
                          />
                        </FormControl>
                        <FormControl w="160px">
                          <Input
                            value={newCategoryCode}
                            onChange={(e) => setNewCategoryCode(e.target.value.toUpperCase())}
                            placeholder={t('assets.categories.code')}
                            maxLength={10}
                            bg={inputBg}
                            borderColor={inputBorderColor}
                            color={textColor}
                            _hover={{ borderColor: inputBorderColor }}
                            _focusVisible={fieldFocusStyles}
                          />
                        </FormControl>
                        {editingCategoryIndex !== null ? (
                          <>
                            <Button
                              colorScheme="blue"
                              onClick={handleUpdateCategory}
                              isDisabled={!newCategoryName.trim()}
                            >
                              {t('common.update')}
                            </Button>
                            <Button
                              variant="outline"
                              onClick={cancelEdit}
                            >
                              {t('common.cancel')}
                            </Button>
                          </>
                        ) : (
                          <Button
                            colorScheme="blue"
                            leftIcon={<FiPlus />}
                            onClick={handleAddCategory}
                            isDisabled={!newCategoryName.trim()}
                          >
                            {t('common.add')}
                          </Button>
                        )}
                      </HStack>
                      <Text fontSize="xs" color={mutedTextColor} mt={2}>
                        💡 {t('assets.categories.codeNote')}
                      </Text>
                    </Box>

                    {/* Existing Categories List */}
                    <Box>
                      <Text fontSize="md" fontWeight="semibold" color={textColor} mb={4}>
                        📋 {t('assets.categories.existingCategories')} ({customCategories.length})
                      </Text>
                      <VStack spacing={2} align="stretch" maxH="300px" overflowY="auto">
                        {customCategories.map((category, index) => {
                          const isUsed = assets.some(asset => asset.category === category);
                          const isDefault = ASSET_CATEGORIES.includes(category as any);

                          return (
                            <HStack
                              key={index}
                              p={3}
                              bg={editingCategoryIndex === index ? selectedCategoryBg : isDefault ? defaultCategoryBg : inputBg}
                              border="1px solid"
                              borderColor={editingCategoryIndex === index ? 'blue.300' : modalBorderColor}
                              borderRadius="md"
                              justify="space-between"
                            >
                              <HStack spacing={3} flex={1}>
                                <Text
                                  fontSize="sm"
                                  fontWeight={editingCategoryIndex === index ? 'semibold' : 'normal'}
                                  color={editingCategoryIndex === index ? selectedCategoryColor : textColor}
                                >
                                  {category}
                                </Text>
                                <Badge colorScheme="purple" size="sm" fontSize="xs">
                                  {getCodeForCategory(category) || '—'}
                                </Badge>
                                <HStack spacing={2}>
                                  {isDefault && (
                                    <Badge colorScheme="gray" size="sm" fontSize="xs">
                                      {t('assets.categories.default')}
                                    </Badge>
                                  )}
                                  {isUsed && (
                                    <Badge colorScheme="green" size="sm" fontSize="xs">
                                      {t('assets.categories.inUse')}
                                    </Badge>
                                  )}
                                </HStack>
                              </HStack>

                              <HStack spacing={2}>
                                <Button
                                  size="sm"
                                  variant="ghost"
                                  leftIcon={<FiEdit />}
                                  onClick={() => handleEditCategory(index)}
                                  isDisabled={editingCategoryIndex !== null && editingCategoryIndex !== index}
                                >
                                  {t('common.edit')}
                                </Button>
                                <Button
                                  size="sm"
                                  variant="ghost"
                                  colorScheme="red"
                                  leftIcon={<FiTrash2 />}
                                  onClick={() => handleDeleteCategory(index)}
                                  isDisabled={isUsed || isDefault}
                                  title={isUsed ? t('assets.categories.cannotDeleteInUse') : isDefault ? t('assets.categories.cannotDeleteDefault') : ''}
                                >
                                  {t('common.delete')}
                                </Button>
                              </HStack>
                            </HStack>
                          );
                        })}

                        {customCategories.length === 0 && (
                          <Box
                            p={6}
                            textAlign="center"
                            bg={emptyStateBg}
                            borderRadius="md"
                            border="2px dashed"
                            borderColor={emptyStateBorder}
                          >
                            <Text color={mutedTextColor}>
                              {t('assets.noAssets')}
                            </Text>
                          </Box>
                        )}
                      </VStack>
                    </Box>
                  </VStack>
                </ModalBody>

                <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={modalBorderColor}>
                  <Button onClick={handleCloseCategoryModal}>
                    {t('common.close')}
                  </Button>
                </ModalFooter>
              </ModalContent>
            </Modal>
          </Box>
        </SimpleLayout>
      </CompanyTypeGuard>
    </ProtectedModule>
  );
};

export default AssetsPage;
