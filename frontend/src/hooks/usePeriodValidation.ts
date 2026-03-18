import { useState, useCallback } from 'react';
import { AxiosError } from 'axios';
import type { UseToastOptions } from '@chakra-ui/react';

export interface PeriodValidationError {
  success: false;
  error: string;
  details: string;
  code: 'PERIOD_CLOSED' | 'PERIOD_LOCKED' | 'PERIOD_NOT_FOUND' | 'DATE_TOO_OLD' | 'DATE_TOO_FUTURE';
  period: string;
}

export interface ReopenPeriodData {
  year: number;
  month: number;
  period: string;
  status: 'CLOSED' | 'LOCKED';
}

export interface UsePeriodValidationOptions {
  onReopenSuccess?: (period: string) => void;
  onReopenError?: (error: any) => void;
  showToast?: boolean;
  toast?: (options: UseToastOptions) => void;
}

export function usePeriodValidation(options: UsePeriodValidationOptions = {}) {
  const { 
    onReopenSuccess, 
    onReopenError,
    showToast = true,
    toast 
  } = options;

  const [reopenDialogOpen, setReopenDialogOpen] = useState(false);
  const [periodToReopen, setPeriodToReopen] = useState<ReopenPeriodData | null>(null);
  const [isReopening, setIsReopening] = useState(false);

  /**
   * Check if error is a period validation error
   */
  const isPeriodValidationError = useCallback((error: any): error is AxiosError<PeriodValidationError> => {
    return (
      error?.response?.status === 403 &&
      error?.response?.data?.code &&
      ['PERIOD_CLOSED', 'PERIOD_LOCKED', 'PERIOD_NOT_FOUND', 'DATE_TOO_OLD', 'DATE_TOO_FUTURE'].includes(
        error.response.data.code
      )
    );
  }, []);

  /**
   * Get user-friendly error message
   */
  const getErrorMessage = useCallback((error: PeriodValidationError): string => {
    const { code, period, details } = error;

    switch (code) {
      case 'PERIOD_CLOSED':
        return `Tidak dapat membuat transaksi: Periode ${period} sudah ditutup.`;
      
      case 'PERIOD_LOCKED':
        return `Tidak dapat membuat transaksi: Periode ${period} telah dikunci secara permanen (fiscal year-end closing).`;
      
      case 'PERIOD_NOT_FOUND':
        return `Periode ${period} tidak ditemukan dan tidak dapat dibuat otomatis.`;
      
      case 'DATE_TOO_OLD':
        return `Tanggal transaksi terlalu lama (lebih dari 2 tahun). Periode tidak dapat dibuat otomatis.`;
      
      case 'DATE_TOO_FUTURE':
        return `Tanggal transaksi terlalu jauh ke depan (lebih dari 7 hari). Gunakan tanggal yang lebih dekat.`;
      
      default:
        return details || 'Period validation error';
    }
  }, []);

  /**
   * Get user-friendly action message
   */
  const getActionMessage = useCallback((error: PeriodValidationError): string => {
    const { code } = error;

    switch (code) {
      case 'PERIOD_CLOSED':
        return 'Anda dapat membuka kembali periode ini jika memiliki permission.';
      
      case 'PERIOD_LOCKED':
        return 'Periode sudah dikunci permanen. Hubungi administrator untuk bantuan.';
      
      case 'DATE_TOO_OLD':
      case 'DATE_TOO_FUTURE':
        return 'Silakan gunakan tanggal yang sesuai atau buat periode secara manual.';
      
      default:
        return '';
    }
  }, []);

  /**
   * Handle period validation error
   */
  const handlePeriodError = useCallback((error: any, onRetry?: () => void) => {
    if (!isPeriodValidationError(error)) {
      return false; // Not a period error, let caller handle it
    }

    const errorData = error.response?.data;
    if (!errorData) {
      return false;
    }
    const errorMessage = getErrorMessage(errorData);
    const actionMessage = getActionMessage(errorData);

    if (showToast && toast) {
      const fullMessage = actionMessage 
        ? `${errorMessage}\n\n${actionMessage}`
        : errorMessage;
      
      toast({
        title: 'Period Validation Error',
        description: fullMessage,
        status: 'error',
        duration: 8000,
        isClosable: true,
        position: 'top',
      });
    }

    // If period is CLOSED (not LOCKED), offer to reopen
    if (errorData.code === 'PERIOD_CLOSED' && errorData.period) {
      const [year, month] = errorData.period.split('-').map(Number);
      
      setPeriodToReopen({
        year,
        month,
        period: errorData.period,
        status: 'CLOSED',
      });
      
      setReopenDialogOpen(true);
    }

    return true; // Error was handled
  }, [isPeriodValidationError, getErrorMessage, getActionMessage, showToast]);

  /**
   * Reopen a closed period
   */
  const reopenPeriod = useCallback(async (
    year: number,
    month: number,
    reason: string,
    apiClient: any // Your axios instance
  ): Promise<boolean> => {
    setIsReopening(true);

    try {
      await apiClient.post(`/periods/${year}/${month}/reopen`, { reason });
      
      if (showToast && toast) {
        toast({
          title: 'Success',
          description: `Periode ${year}-${String(month).padStart(2, '0')} berhasil dibuka kembali.`,
          status: 'success',
          duration: 5000,
          isClosable: true,
          position: 'top',
        });
      }

      if (onReopenSuccess) {
        onReopenSuccess(`${year}-${String(month).padStart(2, '0')}`);
      }

      setReopenDialogOpen(false);
      setPeriodToReopen(null);
      
      return true;
    } catch (error: any) {
      const errorMessage = error?.response?.data?.message || 'Gagal membuka periode';
      
      if (showToast && toast) {
        toast({
          title: 'Error',
          description: `Gagal membuka periode: ${errorMessage}`,
          status: 'error',
          duration: 5000,
          isClosable: true,
          position: 'top',
        });
      }

      if (onReopenError) {
        onReopenError(error);
      }

      return false;
    } finally {
      setIsReopening(false);
    }
  }, [showToast, onReopenSuccess, onReopenError]);

  /**
   * Close reopen dialog
   */
  const closeReopenDialog = useCallback(() => {
    setReopenDialogOpen(false);
    setPeriodToReopen(null);
  }, []);

  return {
    isPeriodValidationError,
    handlePeriodError,
    reopenPeriod,
    reopenDialogOpen,
    periodToReopen,
    isReopening,
    closeReopenDialog,
    getErrorMessage,
    getActionMessage,
  };
}
