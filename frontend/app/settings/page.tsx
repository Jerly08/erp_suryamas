'use client';

import React, { useEffect, useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useTranslation } from '@/hooks/useTranslation';
import { useModulePermissions } from '@/hooks/usePermissions';
import ProtectedModule from '@/components/common/ProtectedModule';
import SimpleLayout from '@/components/layout/SimpleLayout';
import api from '@/services/api';
import { ReopenPeriodDialog } from '@/components/periods/ReopenPeriodDialog';
import { API_ENDPOINTS } from '@/config/api';
import { getImageUrl } from '@/utils/imageUrl';
import {
  Box,
  VStack,
  HStack,
  Heading,
  Text,
  Card,
  CardBody,
  CardHeader,
  SimpleGrid,
  Icon,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Spinner,
  useColorModeValue,
  Divider,
  Select,
  Input,
  Textarea,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  FormControl,
  FormLabel,
  FormErrorMessage,
  useToast,
  Button,
  ButtonGroup,
  Image,
  Stack,
  InputGroup,
  InputLeftElement,
  FormHelperText,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  Tooltip,
  Badge
} from '@chakra-ui/react';
import { FiHome, FiSettings, FiGlobe, FiCalendar, FiDollarSign, FiSave, FiX, FiCreditCard, FiTrendingUp, FiCheckCircle, FiInfo, FiAlertTriangle, FiPackage, FiUnlock, FiClock } from 'react-icons/fi';
import Link from 'next/link';

// Helper for converting fiscal year formats between backend and date input
const MONTHS = [
  'January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'
];

function monthDayStringToISO(src: string, useNextYear?: boolean): string {
  if (!src) return '';
  const lower = src.trim().toLowerCase();
  // Month name + day (e.g., "january 1")
  const monthIdx = MONTHS.findIndex(m => lower.startsWith(m.toLowerCase()));
  if (monthIdx >= 0) {
    const dayMatch = lower.match(/(\d{1,2})/);
    const day = Math.min(Math.max(parseInt(dayMatch?.[1] || '1', 10), 1), 31);

    // Smart year selection: use next year if specified OR if date is in the past
    let year = new Date().getFullYear();
    if (useNextYear) {
      year += 1;
    } else {
      // If the date would be in the past (before today), use next year
      const testDate = new Date(year, monthIdx, day);
      if (testDate < new Date()) {
        year += 1;
      }
    }

    const mm = String(monthIdx + 1).padStart(2, '0');
    const dd = String(day).padStart(2, '0');
    return `${year}-${mm}-${dd}`;
  }
  // Fallback: try parsing any other date-like string
  const d = new Date(src);
  if (!isNaN(d.getTime())) {
    const yyyy = d.getFullYear();
    const mm = String(d.getMonth() + 1).padStart(2, '0');
    const dd = String(d.getDate()).padStart(2, '0');
    return `${yyyy}-${mm}-${dd}`;
  }
  return '';
}

function isoToMonthDayString(iso: string): string {
  if (!iso) return '';
  const parts = iso.split('-');
  if (parts.length < 3) return '';
  const m = parseInt(parts[1], 10);
  const d = parseInt(parts[2], 10);
  if (!m || !d || m < 1 || m > 12) return '';
  return `${MONTHS[m - 1]} ${d}`;
}

// Format an ISO date string to a selected display format
function formatDateISO(iso: string, format: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return '';
  const yyyy = d.getFullYear();
  const mm = String(d.getMonth() + 1).padStart(2, '0');
  const dd = String(d.getDate()).padStart(2, '0');
  switch (format) {
    case 'DD/MM/YYYY':
      return `${dd}/${mm}/${yyyy}`;
    case 'MM/DD/YYYY':
      return `${mm}/${dd}/${yyyy}`;
    case 'DD-MM-YYYY':
      return `${dd}-${mm}-${yyyy}`;
    default:
      return `${yyyy}-${mm}-${dd}`; // 'YYYY-MM-DD'
  }
}

// Compute current fiscal year range from a selected month-day (ISO)
function computeFiscalRange(currentISO: string): { startISO: string; endISO: string } | null {
  if (!currentISO) return null;
  const base = new Date(currentISO);
  if (isNaN(base.getTime())) return null;
  const month = base.getMonth();
  const day = base.getDate();

  const today = new Date();
  const thisYearStart = new Date(Date.UTC(today.getFullYear(), month, day));
  let start = thisYearStart;
  if (today.getTime() < thisYearStart.getTime()) {
    // fiscal year started last year
    start = new Date(Date.UTC(today.getFullYear() - 1, month, day));
  }
  // End date: day before next year's start
  const nextStart = new Date(Date.UTC(start.getUTCFullYear() + 1, month, day));
  const end = new Date(nextStart.getTime() - 24 * 60 * 60 * 1000);

  const toISO = (dt: Date) => `${dt.getUTCFullYear()}-${String(dt.getUTCMonth() + 1).padStart(2, '0')}-${String(dt.getUTCDate()).padStart(2, '0')}`;
  return { startISO: toISO(start), endISO: toISO(end) };
}

interface TaxAccountSettings {
  sales_receivable_account?: { id: number; code: string; name: string };
  sales_revenue_account?: { id: number; code: string; name: string };
  purchase_payable_account?: { id: number; code: string; name: string };
  withholding_tax21_account?: { id: number; code: string; name: string };
  withholding_tax23_account?: { id: number; code: string; name: string };
  withholding_tax25_account?: { id: number; code: string; name: string };
  inventory_account?: { id: number; code: string; name: string };
  cogs_account?: { id: number; code: string; name: string };
  is_active?: boolean;
  updated_at?: string;
}


interface FiscalClosingPreview {
  fiscal_year_end: string;
  total_revenue: number;
  total_expense: number;
  net_income: number;
  retained_earnings_id: number;
  can_close: boolean;
  validation_messages: string[];
  revenue_accounts: Array<{ id: number; code: string; name: string; balance: number }>;
  expense_accounts: Array<{ id: number; code: string; name: string; balance: number }>;
  closing_entries: Array<{ description: string; debit_account: string; credit_account: string; amount: number }>;
}

interface SystemSettings {
  id?: number;
  company_name: string;
  company_address: string;
  company_phone: string;
  company_email: string;
  company_type?: string; // Added field
  company_website?: string;
  company_logo?: string;
  tax_number?: string;
  currency: string;
  date_format: string;
  fiscal_year_start: string;
  default_tax_rate: number;
  language: string;
  timezone: string;
  thousand_separator: string;
  decimal_separator: string;
  decimal_places: number;
  invoice_prefix: string;
  invoice_next_number?: number; // removed from API – keep optional for backward compatibility
  quote_prefix: string;
  quote_next_number?: number; // removed from API – keep optional for backward compatibility
  purchase_prefix: string;
  purchase_next_number?: number; // removed from API – keep optional for backward compatibility
  // New: Payment prefixes
  payment_receivable_prefix?: string;
  payment_payable_prefix?: string;
  email_notifications: boolean;
  smtp_host?: string;
  smtp_port?: number;
  smtp_username?: string;
  smtp_from?: string;
}

const SettingsPage: React.FC = () => {
  const { user } = useAuth();
  const { t, language, setLanguage } = useTranslation();
  const toast = useToast();
  const [settings, setSettings] = useState<SystemSettings | null>(null);
  const [formData, setFormData] = useState<SystemSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);
  const [uploadingLogo, setUploadingLogo] = useState(false);
  // Local UI state for date input binding (ISO format required by <input type="date">)
  const [fiscalStartISO, setFiscalStartISO] = useState<string>('');
  // Tax account settings
  const [taxAccountSettings, setTaxAccountSettings] = useState<TaxAccountSettings | null>(null);
  const [loadingTaxAccounts, setLoadingTaxAccounts] = useState(false);

  // Period closing (flexible period closing)
  const [periodStartDate, setPeriodStartDate] = useState('');
  const [periodEndDate, setPeriodEndDate] = useState('');
  const [periodClosingPreview, setPeriodClosingPreview] = useState<any | null>(null);
  const [loadingPeriodPreview, setLoadingPeriodPreview] = useState(false);
  const [showPeriodClosingModal, setShowPeriodClosingModal] = useState(false);
  const [executingPeriodClosing, setExecutingPeriodClosing] = useState(false);
  const [lastClosingInfo, setLastClosingInfo] = useState<any | null>(null);

  // Reopen period
  const [showReopenDialog, setShowReopenDialog] = useState(false);
  const [reopenStartDate, setReopenStartDate] = useState('');
  const [reopenEndDate, setReopenEndDate] = useState('');

  // Closing history
  const [closingHistory, setClosingHistory] = useState<any[]>([]);
  const [loadingHistory, setLoadingHistory] = useState(false);

  // Move useColorModeValue to top level to fix hooks order
  const blueColor = useColorModeValue('blue.500', 'blue.300');
  const greenColor = useColorModeValue('green.500', 'green.300');
  const orangeColor = useColorModeValue('orange.500', 'orange.300');

  const fetchTaxAccountSettings = async () => {
    setLoadingTaxAccounts(true);
    try {
      const response = await api.get('/api/v1/tax-accounts/current');
      if (response.data.success) {
        setTaxAccountSettings(response.data.data);
      }
    } catch (err: any) {
      console.error('Error fetching tax account settings:', err);
      // Don't show error if tax accounts are not configured yet
    } finally {
      setLoadingTaxAccounts(false);
    }
  };

  // Fetch last closing info to auto-populate start date
  const fetchLastClosingInfo = async () => {
    try {
      const response = await api.get('/api/v1/period-closing/last-info');
      if (response.data.success && response.data.data) {
        const info = response.data.data;
        setLastClosingInfo(info);

        // Auto-populate start date based on last closing
        if (info.has_previous_closing && info.next_start_date) {
          // Has previous closing - MUST use next start date (day after last closing)
          const nextStart = info.next_start_date.split('T')[0];
          setPeriodStartDate(nextStart);
          console.log('✅ Period start date set from last closing:', nextStart);
        } else if (!info.has_previous_closing && info.period_start_date) {
          // First closing - use earliest transaction date
          const firstDate = info.period_start_date.split('T')[0];
          setPeriodStartDate(firstDate);
          console.log('✅ Period start date set from earliest transaction:', firstDate);
        } else {
          // No data available - let user input manually
          console.log('⚠️ No closing info available, user must input start date manually');
        }
      }
    } catch (err) {
      console.error('❌ Failed to fetch last closing info:', err);
      // On error, just let user input dates manually
    }
  };

  // Fetch closing history
  const fetchClosingHistory = async () => {
    setLoadingHistory(true);
    try {
      const response = await api.get('/api/v1/period-closing/history?limit=10');
      if (response.data.success) {
        setClosingHistory(response.data.data || []);
      }
    } catch (err) {
      console.error('Failed to fetch closing history:', err);
    } finally {
      setLoadingHistory(false);
    }
  };

  const handlePreviewPeriodClosing = async () => {
    if (!periodStartDate || !periodEndDate) {
      toast({
        title: 'Dates required',
        description: 'Please select both start and end dates',
        status: 'warning',
        duration: 3000,
      });
      return;
    }

    setLoadingPeriodPreview(true);
    try {
      const response = await api.get(
        `/api/v1/period-closing/preview?start_date=${periodStartDate}&end_date=${periodEndDate}`
      );
      if (response.data.success) {
        setPeriodClosingPreview(response.data.data);
        setShowPeriodClosingModal(true);
      }
    } catch (err: any) {
      toast({
        title: 'Failed to preview closing',
        description: err.response?.data?.details || err.message,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoadingPeriodPreview(false);
    }
  };

  const handleExecutePeriodClosing = async () => {
    if (!periodClosingPreview) return;

    setExecutingPeriodClosing(true);
    try {
      const response = await api.post('/api/v1/period-closing/execute', {
        start_date: periodStartDate,
        end_date: periodEndDate,
        description: `Period closing from ${periodStartDate} to ${periodEndDate}`,
        notes: 'Period closing executed from settings'
      });
      if (response.data.success) {
        toast({
          title: 'Period Closed Successfully!',
          description: 'All revenue and expense accounts have been reset and transferred to retained earnings',
          status: 'success',
          duration: 5000,
          isClosable: true,
        });
        setShowPeriodClosingModal(false);
        setPeriodClosingPreview(null);

        // Refresh settings to update fiscal year start (auto-updated by backend)
        console.log('🔄 Refreshing settings to get updated fiscal_year_start...');
        await fetchSettings();

        // Refresh last closing info
        await fetchLastClosingInfo();

        console.log('✅ Settings and closing info refreshed');
      }
    } catch (err: any) {
      toast({
        title: 'Failed to close period',
        description: err.response?.data?.details || err.message,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setExecutingPeriodClosing(false);
    }
  };

  const fetchSettings = async () => {
    setLoading(true);
    try {
      const response = await api.get(API_ENDPOINTS.SETTINGS);
      if (response.data.success) {
        const fiscalYearStart = response.data.data?.fiscal_year_start;
        const fiscalISO = monthDayStringToISO(fiscalYearStart);

        console.log('📅 Settings loaded:');
        console.log('  - fiscal_year_start (DB):', fiscalYearStart);
        console.log('  - fiscalStartISO (UI):', fiscalISO);

        setSettings(response.data.data);
        setFormData(response.data.data);
        // derive ISO date for UI
        setFiscalStartISO(fiscalISO);
        setHasChanges(false);
        // Sync language from settings
        if (response.data.data.language && response.data.data.language !== language) {
          setLanguage(response.data.data.language);
        }
      }
    } catch (err: any) {
      console.error('Error fetching settings:', err);
      setError(err.response?.data?.error || err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSettings();
    fetchTaxAccountSettings();
    fetchLastClosingInfo();
    fetchClosingHistory();
  }, []);

  // Keep ISO date in sync when settings are populated later
  useEffect(() => {
    if (settings) {
      setFiscalStartISO(prev => prev || monthDayStringToISO(settings.fiscal_year_start));
    }
  }, [settings]);

  const handleLanguageChange = async (newLanguage: string) => {
    if (newLanguage === 'id' || newLanguage === 'en') {
      setLanguage(newLanguage);
      // Update language in backend
      try {
        await api.put(API_ENDPOINTS.SETTINGS_UPDATE, { language: newLanguage });
        toast({
          title: t('settings.languageChanged'),
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      } catch (error) {
        console.error('Error updating language:', error);
      }
    }
  };

  const handleFormChange = (field: keyof SystemSettings, value: any) => {
    setFormData(prev => {
      if (!prev) {
        // If formData is null, create new object with current value
        return {
          ...settings,
          [field]: value
        } as SystemSettings;
      }
      return {
        ...prev,
        [field]: value
      };
    });
    setHasChanges(true);
  };

  const handleSave = async () => {
    if (!formData || !hasChanges) return;

    setSaving(true);
    try {
      const response = await api.put(API_ENDPOINTS.SETTINGS_UPDATE, formData);
      if (response.data.success) {
        setSettings(response.data.data);
        setFormData(response.data.data);
        setHasChanges(false);

        // Update language if changed
        if (response.data.data.language !== language) {
          setLanguage(response.data.data.language);
        }

        toast({
          title: t('settings.updateSuccess') || 'Settings updated successfully',
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      }
    } catch (error: any) {
      toast({
        title: t('settings.updateError') || 'Failed to update settings',
        description: error.response?.data?.details || error.message,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    setFormData(settings);
    setHasChanges(false);
  };

  const handleLogoUpload = async (file: File) => {
    if (!file) return;
    setUploadingLogo(true);
    try {
      const fd = new FormData();
      fd.append('image', file);
      const resp = await api.post('/api/v1/settings/company/logo', fd, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      const newPath: string | undefined = resp.data?.path;
      if (newPath) {
        setSettings(prev => prev ? { ...prev, company_logo: newPath } : prev);
        setFormData(prev => prev ? { ...prev, company_logo: newPath } : prev);
        toast({ title: t('settings.taxAccounts.messages.logoUpdated') || 'Logo updated', status: 'success', duration: 2500, isClosable: true });
      } else {
        toast({ title: t('settings.taxAccounts.messages.uploadSuccessNoPath') || 'Upload succeeded but no path returned', status: 'warning', duration: 3000, isClosable: true });
      }
    } catch (err: any) {
      toast({ title: t('settings.taxAccounts.messages.logoUploadFailed') || 'Failed to upload logo', description: err.response?.data?.error || err.message, status: 'error', duration: 5000, isClosable: true });
    } finally {
      setUploadingLogo(false);
    }
  };


  // Add safety check for formData initialization
  useEffect(() => {
    if (settings && !formData) {
      setFormData(settings);
    }
  }, [settings, formData]);

  // Loading state - moved after all hooks
  if (loading) {
    return (
      <ProtectedModule module="settings">
        <SimpleLayout>
          <Box>
            <Spinner size="xl" thickness="4px" speed="0.65s" color="blue.500" />
            <Text ml={4}>{t('common.loading')}</Text>
          </Box>
        </SimpleLayout>
      </ProtectedModule>
    );
  }

  // Don't render form until settings are loaded
  if (!settings) {
    return (
      <ProtectedModule module="settings">
        <SimpleLayout>
          <Box>
            <Text>No settings data available</Text>
          </Box>
        </SimpleLayout>
      </ProtectedModule>
    );
  }

  return (
    <ProtectedModule module="settings">
      <SimpleLayout>
        <Box>
          <VStack spacing={6} alignItems="start">
            <HStack justify="space-between" width="full">
              <Heading as="h1" size="xl">{t('settings.title')}</Heading>
              {hasChanges && (
                <ButtonGroup>
                  <Button
                    colorScheme="green"
                    leftIcon={<FiSave />}
                    onClick={handleSave}
                    isLoading={saving}
                    loadingText={t('common.saving') || "Saving..."}
                    size="sm"
                  >
                    {t('common.buttons.saveChanges') || "Save Changes"}
                  </Button>
                  <Button
                    variant="outline"
                    leftIcon={<FiX />}
                    onClick={handleCancel}
                    isDisabled={saving}
                    size="sm"
                  >
                    {t('common.cancel') || "Cancel"}
                  </Button>
                </ButtonGroup>
              )}
            </HStack>

            {error && (
              <Alert status="error" width="full">
                <AlertIcon />
                <AlertTitle>{t('common.error')}:</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            <SimpleGrid columns={[1, 1, 2, 2]} spacing={6} width="full">
              {/* Company Information Card */}
              <Card border="1px" borderColor="gray.200" boxShadow="md">
                <CardHeader>
                  <HStack spacing={3}>
                    <Icon as={FiHome} boxSize={6} color={blueColor} />
                    <Heading size="md">{t('settings.companyInfo')}</Heading>
                  </HStack>
                </CardHeader>
                <CardBody>
                  <VStack spacing={4} alignItems="start">
                    {/* Company Logo */}
                    <FormControl>
                      <FormLabel fontWeight="semibold" color="gray.600" fontSize="sm">
                        {t('settings.companyLogo') || "Company Logo"}
                      </FormLabel>
                      <Stack direction={{ base: 'column', md: 'row' }} spacing={4} align="center">
                        <Image
                          src={getImageUrl(formData?.company_logo || settings?.company_logo || '') || undefined}
                          alt="Company Logo"
                          boxSize="80px"
                          objectFit="contain"
                          borderRadius="md"
                          fallbackStrategy="onError"
                        />
                        <Input
                          type="file"
                          accept="image/*"
                          onChange={(e) => {
                            const f = e.target.files?.[0];
                            if (f) handleLogoUpload(f);
                          }}
                          isDisabled={uploadingLogo}
                        />
                        {uploadingLogo && <Spinner size="sm" />}
                      </Stack>
                    </FormControl>
                    <Divider />

                    {/* Company Type (Read Only) */}
                    <FormControl>
                      <FormLabel fontWeight="semibold" color="gray.600" fontSize="sm">
                        {t('settings.companyType') || "Company Type"}
                      </FormLabel>
                      <HStack>
                        <Input
                          value={formData?.company_type ? t(`settings.companyTypes.${formData.company_type.toLowerCase()}`) : t('settings.notSet')}
                          isReadOnly
                          variant="filled"
                          bg="gray.50"
                          color="gray.600"
                          cursor="not-allowed"
                          _hover={{ bg: 'gray.50' }}
                          _focus={{ bg: 'gray.50', borderColor: 'gray.200' }}
                        />
                        {formData?.company_type && (
                          <Tooltip label={t('settings.companyTypeLocked')}>
                            <Box color="gray.400" cursor="help">
                              <Icon as={FiInfo} />
                            </Box>
                          </Tooltip>
                        )}
                      </HStack>
                      {!formData?.company_type && (
                        <FormHelperText fontSize="xs" color="orange.500">
                          {t('settings.companyTypeHelperText')}
                        </FormHelperText>
                      )}
                    </FormControl>
                    <Divider />

                    <FormControl>
                      <FormLabel fontWeight="semibold" color="gray.600" fontSize="sm">
                        {t('settings.companyName')}
                      </FormLabel>
                      <Input
                        value={formData?.company_name ?? settings?.company_name ?? ''}
                        onChange={(e) => handleFormChange('company_name', e.target.value)}
                        placeholder={t('settings.companyNamePlaceholder') || "Enter company name"}
                        variant="filled"
                        _hover={{ bg: 'gray.100' }}
                        _focus={{ bg: 'white', borderColor: 'blue.500' }}
                      />
                    </FormControl>
                    <Divider />

                    <FormControl>
                      <FormLabel fontWeight="semibold" color="gray.600" fontSize="sm">
                        {t('settings.address')}
                      </FormLabel>
                      <Textarea
                        value={formData?.company_address ?? settings?.company_address ?? ''}
                        onChange={(e) => handleFormChange('company_address', e.target.value)}
                        placeholder={t('settings.addressPlaceholder') || "Enter company address"}
                        rows={3}
                        variant="filled"
                        _hover={{ bg: 'gray.100' }}
                        _focus={{ bg: 'white', borderColor: 'blue.500' }}
                      />
                    </FormControl>
                    <Divider />

                    <FormControl>
                      <FormLabel fontWeight="semibold" color="gray.600" fontSize="sm">
                        {t('settings.phone')}
                      </FormLabel>
                      <Input
                        value={formData?.company_phone ?? settings?.company_phone ?? ''}
                        onChange={(e) => handleFormChange('company_phone', e.target.value)}
                        placeholder={t('settings.phonePlaceholder') || "Enter phone number"}
                        type="tel"
                        variant="filled"
                        _hover={{ bg: 'gray.100' }}
                        _focus={{ bg: 'white', borderColor: 'blue.500' }}
                      />
                    </FormControl>
                    <Divider />

                    <FormControl>
                      <FormLabel fontWeight="semibold" color="gray.600" fontSize="sm">
                        {t('settings.email')}
                      </FormLabel>
                      <Input
                        value={formData?.company_email ?? settings?.company_email ?? ''}
                        onChange={(e) => handleFormChange('company_email', e.target.value)}
                        placeholder={t('settings.emailPlaceholder') || "Enter email address"}
                        type="email"
                        variant="filled"
                        _hover={{ bg: 'gray.100' }}
                        _focus={{ bg: 'white', borderColor: 'blue.500' }}
                      />
                    </FormControl>
                    <Divider />

                    <FormControl>
                      <FormLabel fontWeight="semibold" color="gray.600" fontSize="sm">
                        {t('settings.taxNumber')}
                      </FormLabel>
                      <Input
                        value={formData?.tax_number ?? settings?.tax_number ?? ''}
                        onChange={(e) => handleFormChange('tax_number', e.target.value)}
                        placeholder={t('settings.taxNumberPlaceholder') || "Enter tax number (optional)"}
                        variant="filled"
                        _hover={{ bg: 'gray.100' }}
                        _focus={{ bg: 'white', borderColor: 'blue.500' }}
                      />
                    </FormControl>
                  </VStack>
                </CardBody>
              </Card>

              {/* System Configuration Card */}
              <Card border="1px" borderColor="gray.200" boxShadow="md">
                <CardHeader>
                  <HStack spacing={3}>
                    <Icon as={FiDollarSign} boxSize={6} color={greenColor} />
                    <Heading size="md">{t('settings.systemConfig')}</Heading>
                  </HStack>
                </CardHeader>
                <CardBody>
                  <VStack spacing={4} alignItems="start">
                    <FormControl>
                      <FormLabel fontWeight="semibold" color="gray.600" fontSize="sm">
                        {t('settings.dateFormat')}
                      </FormLabel>
                      <Select
                        value={formData?.date_format || settings?.date_format || 'YYYY-MM-DD'}
                        onChange={(e) => handleFormChange('date_format', e.target.value)}
                        variant="filled"
                        _hover={{ bg: 'gray.100' }}
                        _focus={{ bg: 'white', borderColor: 'blue.500' }}
                      >
                        <option value="YYYY-MM-DD">YYYY-MM-DD</option>
                        <option value="DD/MM/YYYY">DD/MM/YYYY</option>
                        <option value="MM/DD/YYYY">MM/DD/YYYY</option>
                        <option value="DD-MM-YYYY">DD-MM-YYYY</option>
                      </Select>
                    </FormControl>
                    <Divider />

                    <FormControl>
                      <FormLabel fontWeight="semibold" color="gray.600" fontSize="sm">
                        {t('settings.defaultTaxRate')}
                      </FormLabel>
                      <NumberInput
                        value={formData?.default_tax_rate ?? settings?.default_tax_rate ?? 0}
                        onChange={(valueString) => handleFormChange('default_tax_rate', parseFloat(valueString) || 0)}
                        min={0}
                        max={100}
                        precision={2}
                      >
                        <NumberInputField
                          variant="filled"
                          _hover={{ bg: 'gray.100' }}
                          _focus={{ bg: 'white', borderColor: 'blue.500' }}
                        />
                        <NumberInputStepper>
                          <NumberIncrementStepper />
                          <NumberDecrementStepper />
                        </NumberInputStepper>
                      </NumberInput>
                    </FormControl>
                    <Divider />

                    <FormControl>
                      <HStack mb={2}>
                        <Icon as={FiGlobe} boxSize={4} color="gray.600" />
                        <FormLabel fontWeight="semibold" color="gray.600" fontSize="sm" mb={0}>
                          {t('settings.language')}
                        </FormLabel>
                      </HStack>
                      <Select
                        value={formData?.language || settings?.language || language}
                        onChange={(e) => {
                          handleFormChange('language', e.target.value);
                        }}
                        variant="filled"
                        _hover={{ bg: 'gray.100' }}
                        _focus={{ bg: 'white', borderColor: 'blue.500' }}
                      >
                        <option value="id">{t('settings.indonesian')}</option>
                        <option value="en">{t('settings.english')}</option>
                      </Select>
                    </FormControl>
                  </VStack>
                </CardBody>
              </Card>

              {/* Flexible Period Closing Card */}
              <Card border="1px" borderColor="gray.200" boxShadow="md">
                <CardHeader>
                  <HStack spacing={3} justify="space-between" width="full">
                    <HStack spacing={3}>
                      <Icon as={FiCheckCircle} boxSize={6} color="red.500" />
                      <Heading size="md">{t('settings.periodClosing') || 'Period Closing'}</Heading>
                    </HStack>
                    <Tooltip
                      label={
                        <Box p={2}>
                          <Text fontSize="xs" fontWeight="bold" mb={1}>
                            📘 {t('settings.periodClosingDesc')}
                          </Text>
                          <Text fontSize="xs" fontWeight="semibold" mb={1}>
                            {t('settings.periodClosingFeatures')}
                          </Text>
                          <Text fontSize="xs" color="green.200">• {t('settings.periodClosingResetRev')}</Text>
                          <Text fontSize="xs" color="red.200">• {t('settings.periodClosingResetExp')}</Text>
                          <Text fontSize="xs" color="blue.200">• {t('settings.periodClosingRetained')}</Text>
                          <Text fontSize="xs">• {t('settings.periodClosingJournal')}</Text>
                          <Text fontSize="xs">• {t('settings.periodClosingLock')}</Text>
                          <Text fontSize="xs" mt={2} fontWeight="semibold" color="orange.200">
                            {t('settings.permanentAction')}
                          </Text>
                        </Box>
                      }
                      placement="left"
                      hasArrow
                      bg="red.600"
                    >
                      <Box cursor="help">
                        <Icon as={FiInfo} boxSize={4} color="gray.500" _hover={{ color: 'red.500' }} />
                      </Box>
                    </Tooltip>
                  </HStack>
                </CardHeader>
                <CardBody>
                  <VStack spacing={4} alignItems="start">
                    <Alert status="error" borderRadius="md" size="sm">
                      <AlertIcon />
                      <Box>
                        <AlertTitle fontSize="sm">⚠️ {t('settings.permanentAction')}</AlertTitle>
                        <AlertDescription fontSize="xs">
                          {t('settings.periodClosingWarnings.permanentActionDesc')}
                        </AlertDescription>
                      </Box>
                    </Alert>

                    {lastClosingInfo?.has_previous_closing && lastClosingInfo?.last_closing_date && (
                      <Alert status="info" borderRadius="md" size="sm">
                        <AlertIcon />
                        <Box flex="1">
                          <AlertTitle fontSize="sm">📅 {t('settings.periodClosingLabels.lastClosingTitle')}</AlertTitle>
                          <AlertDescription fontSize="xs">
                            {t('settings.periodClosingLabels.lastClosed')}: {new Date(lastClosingInfo.last_closing_date).toLocaleDateString()}
                          </AlertDescription>
                        </Box>
                        <Button
                          size="xs"
                          colorScheme="orange"
                          variant="solid"
                          leftIcon={<FiUnlock />}
                          onClick={() => {
                            setReopenStartDate(lastClosingInfo.last_period_start_date || lastClosingInfo.last_closing_date);
                            setReopenEndDate(lastClosingInfo.last_period_end_date || lastClosingInfo.last_closing_date);
                            setShowReopenDialog(true);
                          }}
                          ml={2}
                        >
                          Buka Kembali
                        </Button>
                      </Alert>
                    )}

                    {lastClosingInfo && !lastClosingInfo.has_previous_closing && (
                      <Alert status="warning" borderRadius="md" size="sm">
                        <AlertIcon />
                        <Box>
                          <AlertTitle fontSize="sm">⚠️ {t('settings.periodClosingWarnings.firstTimeTitle')}</AlertTitle>
                          <AlertDescription fontSize="xs">
                            {t('settings.periodClosingWarnings.firstTimeDesc')}
                          </AlertDescription>
                        </Box>
                      </Alert>
                    )}

                    <Alert status="success" borderRadius="md" size="sm" variant="left-accent">
                      <AlertIcon />
                      <Box>
                        <AlertTitle fontSize="sm">🔄 {t('settings.periodClosingWarnings.autoUpdateTitle')}</AlertTitle>
                        <AlertDescription fontSize="xs">
                          <Box dangerouslySetInnerHTML={{ __html: t('settings.periodClosingWarnings.autoUpdateDesc') }} />
                        </AlertDescription>
                      </Box>
                    </Alert>

                    <Text fontSize="sm" color="gray.600">
                      {t('settings.periodClosingLabels.description')}
                    </Text>

                    <FormControl>
                      <FormLabel fontSize="sm" fontWeight="semibold">
                        {t('common.filters.startDate') || "Start Date"}
                      </FormLabel>
                      <Input
                        type="date"
                        value={periodStartDate}
                        onChange={(e) => setPeriodStartDate(e.target.value)}
                        variant="filled"
                        bg={lastClosingInfo?.has_previous_closing ? 'blue.50' : undefined}
                        _hover={{ bg: lastClosingInfo?.has_previous_closing ? 'blue.50' : 'gray.100' }}
                        _focus={{ bg: 'white', borderColor: 'blue.500' }}
                        isReadOnly={lastClosingInfo?.has_previous_closing === true}
                        cursor={lastClosingInfo?.has_previous_closing ? 'not-allowed' : 'text'}
                        placeholder="Select start date"
                      />
                      <FormHelperText fontSize="xs" color={lastClosingInfo?.has_previous_closing ? 'blue.600' : 'gray.600'}>
                        {lastClosingInfo?.has_previous_closing
                          ? `🔒 ${t('settings.periodClosingHelper.autoStart')} (${lastClosingInfo.last_closing_date ? new Date(lastClosingInfo.last_closing_date).toLocaleDateString(language === 'id' ? 'id-ID' : 'en-US') : 'N/A'})`
                          : `📅 ${t('settings.periodClosingHelper.manualStart')}`}
                      </FormHelperText>
                    </FormControl>

                    <FormControl>
                      <FormLabel fontSize="sm" fontWeight="semibold">
                        {t('common.filters.endDate') || "End Date"}
                      </FormLabel>
                      <Input
                        type="date"
                        value={periodEndDate}
                        onChange={(e) => setPeriodEndDate(e.target.value)}
                        variant="filled"
                        _hover={{ bg: 'gray.100' }}
                        _focus={{ bg: 'white', borderColor: 'blue.500' }}
                      />
                      <FormHelperText fontSize="xs">
                        {t('settings.periodClosingLabels.endDateHelper')}
                      </FormHelperText>
                    </FormControl>

                    <Button
                      colorScheme="red"
                      variant="solid"
                      width="full"
                      onClick={handlePreviewPeriodClosing}
                      isLoading={loadingPeriodPreview}
                      loadingText={t('settings.periodClosingLabels.loadingPreview')}
                      isDisabled={!periodStartDate || !periodEndDate}
                    >
                      {t('settings.periodClosingLabels.previewButton')}
                    </Button>

                    {/* Closing History Section */}
                    <Divider />
                    <HStack justify="space-between" width="full">
                      <HStack spacing={2}>
                        <Icon as={FiClock} color="gray.600" />
                        <Text fontWeight="semibold" fontSize="sm">Riwayat Tutup Buku</Text>
                      </HStack>
                      <Button
                        size="xs"
                        variant="ghost"
                        onClick={fetchClosingHistory}
                        isLoading={loadingHistory}
                      >
                        Refresh
                      </Button>
                    </HStack>

                    {loadingHistory ? (
                      <HStack justify="center" py={3}>
                        <Spinner size="sm" />
                        <Text fontSize="xs">Loading...</Text>
                      </HStack>
                    ) : closingHistory.length === 0 ? (
                      <Text fontSize="xs" color="gray.500" textAlign="center" py={2}>
                        Belum ada riwayat tutup buku
                      </Text>
                    ) : (
                      <VStack spacing={2} width="full" alignItems="stretch">
                        {closingHistory.map((item: any) => (
                          <Box
                            key={item.id}
                            p={3}
                            borderRadius="md"
                            border="1px"
                            borderColor={item.is_closed ? 'green.200' : 'orange.200'}
                            bg={item.is_closed ? 'green.50' : 'orange.50'}
                          >
                            <HStack justify="space-between" mb={1}>
                              <HStack spacing={2}>
                                <Badge
                                  colorScheme={item.is_closed ? 'green' : 'orange'}
                                  fontSize="2xs"
                                >
                                  {item.is_closed ? '🔒 Tertutup' : '🔓 Terbuka'}
                                </Badge>
                                <Text fontSize="xs" fontWeight="semibold">
                                  {item.start_date && !item.start_date.startsWith('0001')
                                    ? new Date(item.start_date).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' })
                                    : '—'}
                                  {' — '}
                                  {item.end_date && !item.end_date.startsWith('0001')
                                    ? new Date(item.end_date).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' })
                                    : '—'}
                                </Text>
                              </HStack>
                              {item.is_closed && (
                                <Button
                                  size="xs"
                                  variant="outline"
                                  colorScheme="orange"
                                  leftIcon={<FiUnlock />}
                                  onClick={() => {
                                    setReopenStartDate(item.start_date);
                                    setReopenEndDate(item.end_date);
                                    setShowReopenDialog(true);
                                  }}
                                >
                                  Buka Kembali
                                </Button>
                              )}
                            </HStack>
                            <HStack spacing={4} fontSize="xs" color="gray.600">
                              <Text>Revenue: Rp {(item.total_revenue || 0).toLocaleString('id-ID')}</Text>
                              <Text>Expense: Rp {(item.total_expense || 0).toLocaleString('id-ID')}</Text>
                              <Text fontWeight="semibold" color={item.net_income >= 0 ? 'green.600' : 'red.600'}>
                                Net: Rp {(item.net_income || 0).toLocaleString('id-ID')}
                              </Text>
                            </HStack>
                            {
                              item.notes && (
                                <Text fontSize="xs" color="gray.500" mt={1} fontStyle="italic">
                                  {item.notes}
                                </Text>
                              )
                            }
                          </Box>
                        ))}
                      </VStack>
                    )}
                  </VStack>
                </CardBody>
              </Card>

              {/* Tax Account Configuration Card */}
              <Card border="1px" borderColor="gray.200" boxShadow="md">
                <CardHeader>
                  <HStack spacing={3} justify="space-between">
                    <HStack spacing={3}>
                      <Icon as={FiCreditCard} boxSize={6} color={orangeColor} />
                      <Heading size="md">{t('settings.taxAccountConfig')}</Heading>
                    </HStack>
                    {settings?.company_type && (
                      <Badge
                        colorScheme={
                          settings.company_type === 'SERVICE' ? 'purple' :
                            settings.company_type === 'TRADING' ? 'teal' : 'blue'
                        }
                        fontSize="xs"
                        px={3}
                        py={1}
                        borderRadius="full"
                        textTransform="uppercase"
                      >
                        {settings.company_type === 'SERVICE' ? t('settings.companyTypeBadges.service') :
                          settings.company_type === 'TRADING' ? t('settings.companyTypeBadges.trading') : t('settings.companyTypeBadges.manufacturing')}
                      </Badge>
                    )}
                  </HStack>
                </CardHeader>
                <CardBody>
                  <VStack spacing={4} alignItems="start">
                    <Text fontSize="sm" color="gray.600" mb={2}>
                      {t('settings.taxAccountConfigDesc')}
                    </Text>

                    {/* PPN Rate Display */}
                    <Box width="full" p={3} bg="orange.50" borderRadius="md" border="1px" borderColor="orange.200">
                      <HStack justify="space-between">
                        <HStack spacing={2}>
                          <Icon as={FiDollarSign} color="orange.600" boxSize={4} />
                          <Text fontSize="sm" fontWeight="semibold" color="orange.800">{t('settings.taxRecommendations.ppnLabel')}</Text>
                        </HStack>
                        <Badge colorScheme="orange" fontSize="sm" px={2} py={1} borderRadius="md">
                          {settings?.default_tax_rate ?? 11}%
                        </Badge>
                      </HStack>
                    </Box>

                    {/* Company-type-specific recommendation */}
                    {settings?.company_type === 'SERVICE' && (
                      <Alert status="info" variant="left-accent" size="sm" borderRadius="md">
                        <AlertIcon />
                        <AlertDescription fontSize="xs">
                          💡 {t('settings.taxRecommendations.service')}
                        </AlertDescription>
                      </Alert>
                    )}
                    {settings?.company_type === 'TRADING' && (
                      <Alert status="warning" variant="left-accent" size="sm" borderRadius="md">
                        <AlertIcon />
                        <AlertDescription fontSize="xs">
                          <Box dangerouslySetInnerHTML={{ __html: `📦 ${t('settings.taxRecommendations.trading')}` }} />
                        </AlertDescription>
                      </Alert>
                    )}
                    {settings?.company_type === 'MANUFACTURING' && (
                      <Alert status="warning" variant="left-accent" size="sm" borderRadius="md">
                        <AlertIcon />
                        <AlertDescription fontSize="xs">
                          <Box dangerouslySetInnerHTML={{ __html: `🏭 ${t('settings.taxRecommendations.manufacturing')}` }} />
                        </AlertDescription>
                      </Alert>
                    )}
                    {!settings?.company_type && (
                      <Alert status="error" variant="left-accent" size="sm" borderRadius="md">
                        <AlertIcon />
                        <AlertDescription fontSize="xs">
                          ⚠️ {t('settings.taxRecommendations.noType')}
                        </AlertDescription>
                      </Alert>
                    )}

                    {loadingTaxAccounts ? (
                      <HStack width="full" justify="center" py={4}>
                        <Spinner size="sm" color="blue.500" />
                        <Text fontSize="xs" color="gray.500">{t('settings.taxAccountConfigLoading')}</Text>
                      </HStack>
                    ) : taxAccountSettings ? (
                      <VStack spacing={3} width="full" alignItems="start">
                        {/* Withholding Tax Section */}
                        <HStack justify="space-between" width="full">
                          <VStack alignItems="start" spacing={1}>
                            <HStack spacing={2}>
                              <Text fontSize="sm" fontWeight="semibold" color="gray.700">
                                {t('settings.taxAccountSections.withholding')}
                              </Text>
                              {settings?.company_type === 'SERVICE' && (
                                <Badge colorScheme="red" fontSize="2xs" variant="subtle">{t('settings.taxRecommendations.primary')}</Badge>
                              )}
                            </HStack>
                            <VStack spacing={1} alignItems="start" mt={2}>
                              {taxAccountSettings.withholding_tax21_account ? (
                                <HStack spacing={1}>
                                  <Icon as={FiCheckCircle} color="green.500" boxSize={3} />
                                  <Text fontSize="xs" color="green.600" fontWeight="medium">
                                    PPh 21: {taxAccountSettings.withholding_tax21_account.code} - {taxAccountSettings.withholding_tax21_account.name}
                                  </Text>
                                </HStack>
                              ) : (
                                <HStack spacing={1}>
                                  <Icon as={settings?.company_type === 'SERVICE' ? FiAlertTriangle : FiInfo} color={settings?.company_type === 'SERVICE' ? 'orange.400' : 'gray.400'} boxSize={3} />
                                  <Text fontSize="xs" color={settings?.company_type === 'SERVICE' ? 'orange.500' : 'gray.400'}>PPh 21: {t('settings.taxAccountStatus.notConfigured')}</Text>
                                </HStack>
                              )}
                              {taxAccountSettings.withholding_tax23_account ? (
                                <HStack spacing={1}>
                                  <Icon as={FiCheckCircle} color="green.500" boxSize={3} />
                                  <Text fontSize="xs" color="green.600" fontWeight="medium">
                                    PPh 23: {taxAccountSettings.withholding_tax23_account.code} - {taxAccountSettings.withholding_tax23_account.name}
                                  </Text>
                                </HStack>
                              ) : (
                                <HStack spacing={1}>
                                  <Icon as={(settings?.company_type === 'SERVICE' || settings?.company_type === 'MANUFACTURING') ? FiAlertTriangle : FiInfo} color={(settings?.company_type === 'SERVICE' || settings?.company_type === 'MANUFACTURING') ? 'orange.400' : 'gray.400'} boxSize={3} />
                                  <Text fontSize="xs" color={(settings?.company_type === 'SERVICE' || settings?.company_type === 'MANUFACTURING') ? 'orange.500' : 'gray.400'}>PPh 23: {t('settings.taxAccountStatus.notConfigured')}</Text>
                                </HStack>
                              )}
                              {taxAccountSettings.withholding_tax25_account ? (
                                <HStack spacing={1}>
                                  <Icon as={FiCheckCircle} color="green.500" boxSize={3} />
                                  <Text fontSize="xs" color="green.600" fontWeight="medium">
                                    PPh 25: {taxAccountSettings.withholding_tax25_account.code} - {taxAccountSettings.withholding_tax25_account.name}
                                  </Text>
                                </HStack>
                              ) : (
                                <Text fontSize="xs" color="gray.400">PPh 25: {t('settings.taxAccountStatus.notConfigured')} ({t('settings.taxRecommendations.optional')})</Text>
                              )}
                            </VStack>
                          </VStack>
                          <Icon as={FiDollarSign} color="orange.500" boxSize={4} />
                        </HStack>

                        {/* Inventory & COGS Section - conditional based on company type */}
                        {settings?.company_type !== 'SERVICE' && (
                          <>
                            <Divider />
                            <HStack justify="space-between" width="full">
                              <VStack alignItems="start" spacing={1}>
                                <HStack spacing={2}>
                                  <Text fontSize="sm" fontWeight="semibold" color="gray.700">
                                    {t('settings.taxAccountSections.inventoryCogs')}
                                  </Text>
                                  {(settings?.company_type === 'TRADING' || settings?.company_type === 'MANUFACTURING') && (
                                    <Badge colorScheme="red" fontSize="2xs" variant="subtle">{t('settings.taxRecommendations.required')}</Badge>
                                  )}
                                </HStack>
                                <VStack spacing={1} alignItems="start" mt={2}>
                                  {taxAccountSettings.inventory_account ? (
                                    <HStack spacing={1}>
                                      <Icon as={FiCheckCircle} color="green.500" boxSize={3} />
                                      <Text fontSize="xs" color="green.600" fontWeight="medium">
                                        Inventory: {taxAccountSettings.inventory_account.code} - {taxAccountSettings.inventory_account.name}
                                      </Text>
                                    </HStack>
                                  ) : (
                                    <HStack spacing={1}>
                                      <Icon as={FiAlertTriangle} color="red.400" boxSize={3} />
                                      <Text fontSize="xs" color="red.500" fontWeight="medium">Inventory: {t('settings.taxAccountStatus.notConfigured')} ⚠️</Text>
                                    </HStack>
                                  )}
                                  {taxAccountSettings.cogs_account ? (
                                    <HStack spacing={1}>
                                      <Icon as={FiCheckCircle} color="green.500" boxSize={3} />
                                      <Text fontSize="xs" color="green.600" fontWeight="medium">
                                        COGS: {taxAccountSettings.cogs_account.code} - {taxAccountSettings.cogs_account.name}
                                      </Text>
                                    </HStack>
                                  ) : (
                                    <HStack spacing={1}>
                                      <Icon as={FiAlertTriangle} color="red.400" boxSize={3} />
                                      <Text fontSize="xs" color="red.500" fontWeight="medium">COGS: {t('settings.taxAccountStatus.notConfigured')} ⚠️</Text>
                                    </HStack>
                                  )}
                                </VStack>
                              </VStack>
                              <Icon as={FiPackage} color="blue.500" boxSize={4} />
                            </HStack>
                          </>
                        )}
                      </VStack>
                    ) : (
                      <VStack spacing={3} width="full" alignItems="start">
                        <Text fontSize="xs" color="gray.500">
                          {t('settings.taxAccountStatus.noConfigFound')}
                        </Text>
                      </VStack>
                    )}

                    <Link href="/settings/tax-accounts">
                      <Button
                        colorScheme="orange"
                        variant="outline"
                        size="sm"
                        leftIcon={<FiSettings />}
                        width="full"
                        _hover={{ bg: 'orange.50' }}
                      >
                        {t('settings.taxAccounts.title') || "Configure Tax Accounts"}
                      </Button>
                    </Link>
                  </VStack>
                </CardBody>
              </Card>
            </SimpleGrid>
          </VStack>
        </Box>

        {/* Period Closing Preview Modal */}
        <Modal
          isOpen={showPeriodClosingModal}
          onClose={() => setShowPeriodClosingModal(false)}
          size="xl"
        >
          <ModalOverlay />
          <ModalContent maxW="800px">
            <ModalHeader bg="red.500" color="white">
              📅 {t('settings.periodClosing') || "Period Closing Preview"}
            </ModalHeader>
            <ModalCloseButton color="white" />
            <ModalBody py={6}>
              {periodClosingPreview && (
                <VStack spacing={4} alignItems="start">
                  {/* Period Info */}
                  <Box width="full" p={3} bg="blue.50" borderRadius="md">
                    <Text fontSize="sm" fontWeight="bold" mb={1}>
                      {t('settings.periodClosingLabels.period')}: {new Date(periodClosingPreview.start_date).toLocaleDateString()} - {new Date(periodClosingPreview.end_date).toLocaleDateString()}
                    </Text>
                    <Text fontSize="xs" color="gray.600">
                      {periodClosingPreview.period_days} {t('settings.periodClosingLabels.days')} | {periodClosingPreview.transaction_count} {t('settings.periodClosingLabels.transactions')}
                    </Text>
                  </Box>

                  {/* Summary */}
                  <Box width="full" p={4} bg="gray.50" borderRadius="md">
                    <Text fontSize="lg" fontWeight="bold" mb={2}>
                      {t('settings.periodClosingLabels.financialSummary')}
                    </Text>
                    <SimpleGrid columns={3} spacing={4}>
                      <Box>
                        <Text fontSize="xs" color="gray.600">{t('settings.periodClosingLabels.totalRevenue')}</Text>
                        <Text fontSize="lg" fontWeight="bold" color="green.600">
                          Rp {periodClosingPreview.total_revenue.toLocaleString('id-ID')}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontSize="xs" color="gray.600">{t('settings.periodClosingLabels.totalExpense')}</Text>
                        <Text fontSize="lg" fontWeight="bold" color="red.600">
                          Rp {periodClosingPreview.total_expense.toLocaleString('id-ID')}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontSize="xs" color="gray.600">{t('settings.periodClosingLabels.netIncome')}</Text>
                        <Text fontSize="lg" fontWeight="bold" color="blue.600">
                          Rp {periodClosingPreview.net_income.toLocaleString('id-ID')}
                        </Text>
                      </Box>
                    </SimpleGrid>
                  </Box>

                  {/* Validation Messages */}
                  {periodClosingPreview.validation_messages && periodClosingPreview.validation_messages.length > 0 && (
                    <Alert status={periodClosingPreview.can_close ? 'info' : 'error'} borderRadius="md">
                      <AlertIcon />
                      <Box>
                        <AlertTitle fontSize="sm">{periodClosingPreview.can_close ? t('settings.periodClosingLabels.validation') : t('settings.periodClosingLabels.validationIssues')}</AlertTitle>
                        <AlertDescription fontSize="xs">
                          {periodClosingPreview.validation_messages.map((msg: string, i: number) => (
                            <Text key={i}>{msg}</Text>
                          ))}
                        </AlertDescription>
                      </Box>
                    </Alert>
                  )}

                  {/* What Will Happen */}
                  {periodClosingPreview.can_close && (
                    <Alert status="warning" borderRadius="md">
                      <AlertIcon />
                      <Box>
                        <AlertTitle fontSize="sm">⚠️ {t('settings.periodClosingFeatures') || "What will happen:"}</AlertTitle>
                        <AlertDescription fontSize="xs">
                          • {periodClosingPreview.revenue_accounts?.length || 0} {t('settings.periodClosingLabels.featureRevenue')}
                          <br />
                          • {periodClosingPreview.expense_accounts?.length || 0} {t('settings.periodClosingLabels.featureExpense')}
                          <br />
                          • {t('settings.periodClosingLabels.featureRetained')} Rp {periodClosingPreview.net_income.toLocaleString('id-ID')}
                          <br />
                          • {t('settings.periodClosingLabels.featureJournal')}
                          <br />
                          • {t('settings.periodClosingLabels.featureLocked')}
                        </AlertDescription>
                      </Box>
                    </Alert>
                  )}

                  {/* Closing Entries Preview */}
                  {periodClosingPreview.closing_entries && periodClosingPreview.closing_entries.length > 0 && (
                    <Box width="full">
                      <Text fontSize="sm" fontWeight="bold" mb={2}>
                        {t('settings.periodClosingLabels.closingEntries')}
                      </Text>
                      <VStack spacing={2} width="full" alignItems="start">
                        {periodClosingPreview.closing_entries.map((entry: any, i: number) => (
                          <Box
                            key={i}
                            width="full"
                            p={3}
                            bg="white"
                            border="1px"
                            borderColor="gray.200"
                            borderRadius="md"
                          >
                            <Text fontSize="xs" fontWeight="semibold" mb={1}>
                              {entry.description}
                            </Text>
                            <HStack spacing={2} fontSize="xs">
                              <Text>Dr: {entry.debit_account}</Text>
                              <Text>→</Text>
                              <Text>Cr: {entry.credit_account}</Text>
                              <Text fontWeight="bold">
                                Rp {entry.amount.toLocaleString('id-ID')}
                              </Text>
                            </HStack>
                          </Box>
                        ))}
                      </VStack>
                    </Box>
                  )}
                </VStack>
              )}
            </ModalBody>
            <ModalFooter>
              <Button
                variant="ghost"
                mr={3}
                onClick={() => setShowPeriodClosingModal(false)}
                isDisabled={executingPeriodClosing}
              >
                {t('common.cancel') || "Cancel"}
              </Button>
              <Button
                colorScheme="red"
                onClick={handleExecutePeriodClosing}
                isLoading={executingPeriodClosing}
                loadingText={t('common.processing') || "Closing..."}
                isDisabled={!periodClosingPreview?.can_close || executingPeriodClosing}
              >
                {t('settings.periodClosing') || "Execute Period Closing"}
              </Button>
            </ModalFooter>
          </ModalContent>
        </Modal>

        {/* Reopen Period Dialog */}
        <ReopenPeriodDialog
          isOpen={showReopenDialog}
          onClose={() => setShowReopenDialog(false)}
          startDate={reopenStartDate}
          endDate={reopenEndDate}
          onSuccess={() => {
            fetchLastClosingInfo();
            fetchClosingHistory();
          }}
        />

      </SimpleLayout>
    </ProtectedModule >
  );
};

export default SettingsPage;
