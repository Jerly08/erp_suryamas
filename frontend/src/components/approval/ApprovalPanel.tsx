import React, { useState } from 'react';
import { Box, VStack, HStack, Badge, Text, Spinner, Alert, AlertIcon, Divider, Button, Textarea, FormControl, FormLabel, FormHelperText, Checkbox, useToast, useColorModeValue } from '@chakra-ui/react';
import { useApproval } from '@/hooks/useApproval';
import { useAuth } from '@/contexts/AuthContext';
import approvalService from '@/services/approvalService';
import { normalizeRole, formatRoleForApproval } from '@/utils/roles';

interface ApprovalPanelProps {
  purchaseId?: number;
  approvalStatus?: string;
  canApprove?: boolean;
  onApprove?: (comments?: string, requiresDirector?: boolean) => void;
  onReject?: (comments: string) => void;
  actionLoading?: boolean;
  purchaseAmount?: number;
}

const statusColor = (s?: string) => {
  switch ((s || '').toUpperCase()) {
    case 'APPROVED':
      return 'green';
    case 'PENDING':
    case 'PENDING_APPROVAL':
      return 'yellow';
    case 'REJECTED':
      return 'red';
    default:
      return 'gray';
  }
};

export const ApprovalPanel: React.FC<ApprovalPanelProps> = ({ purchaseId, approvalStatus, canApprove = false, onApprove, onReject, actionLoading = false, purchaseAmount = 0 }) => {
  const { history, loading, error, refresh } = useApproval(purchaseId);
  const { user } = useAuth();
  const toast = useToast();
  const [comments, setComments] = useState('');
  const [requiresDirector, setRequiresDirector] = useState(false);
  const [hasUserInteracted, setHasUserInteracted] = useState(false);
  const [validationAttempted, setValidationAttempted] = useState(false);
  const [approvalIntent, setApprovalIntent] = useState<'approve' | 'reject' | null>(null);
  const panelBg = useColorModeValue('white', 'gray.800');
  const panelBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const panelHeadingColor = useColorModeValue('gray.800', 'whiteAlpha.900');
  const mutedTextColor = useColorModeValue('gray.500', 'whiteAlpha.700');
  const bodyTextColor = useColorModeValue('gray.700', 'whiteAlpha.900');
  const historyCardBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const historyRejectedBg = useColorModeValue('red.50', 'rgba(245, 101, 101, 0.16)');
  const roleTextColor = useColorModeValue('blue.600', 'blue.200');
  const dividerColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const textAreaBg = useColorModeValue('white', 'whiteAlpha.50');
  const textAreaBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
  const textAreaHoverColor = useColorModeValue('gray.400', 'whiteAlpha.500');
  const directorHelperColor = useColorModeValue('gray.600', 'whiteAlpha.700');
  const directorHintColor = useColorModeValue('blue.600', 'blue.200');
  const rejectHoverShadow = useColorModeValue('0 4px 12px rgba(245, 101, 101, 0.25)', '0 4px 12px rgba(245, 101, 101, 0.18)');
  const approveHoverShadow = useColorModeValue('0 4px 12px rgba(72, 187, 120, 0.25)', '0 4px 12px rgba(72, 187, 120, 0.18)');

  // Enhanced helper functions for UX
  const getValidationMessage = () => {
    if (hasUserInteracted && !comments.trim()) {
      return {
        severity: 'warning' as const,
        message: 'Alasan penolakan diperlukan sesuai prosedur SOP.'
      };
    }
    if (validationAttempted && !comments.trim()) {
      return {
        severity: 'error' as const,
        message: 'Komentar wajib diisi untuk penolakan.'
      };
    }
    return null;
  };

  const isFormValid = () => {
    return comments.trim().length > 0;
  };

  const getButtonOpacity = () => {
    if (!isFormValid() && hasUserInteracted) {
      return 0.6;
    }
    return 1;
  };

  const handleCommentsChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const newValue = e.target.value;
    setComments(newValue);
    
    // Reset approval intent when comments change
    if (approvalIntent && newValue.trim() === '') {
      setApprovalIntent(null);
    }
    
    if (!hasUserInteracted) {
      setHasUserInteracted(true);
    }
    if (validationAttempted) {
      setValidationAttempted(false);
    }
  };

  const rejectDisabled = comments.trim() === '';
  const userRole = user?.role || '';
  const normalizedRole = normalizeRole(userRole as any);
  const isFinanceRole = normalizedRole === 'finance';
  const isDirectorRole = normalizedRole === 'director';
  const isPendingApproval = (approvalStatus || '').toUpperCase() === 'PENDING';
  const isNotStarted = (approvalStatus || '').toUpperCase() === 'NOT_STARTED';
  const isApproved = (approvalStatus || '').toUpperCase() === 'APPROVED';
  const isRejected = (approvalStatus || '').toUpperCase() === 'REJECTED';
  
  // Determine the active step based on approval history
  const getActiveApprovalStep = () => {
    if (isApproved || isRejected) {
      return null; // Process completed
    }
    
    // For new approval workflow starting from Employee
    if (history.length === 0 || isNotStarted) {
      // Employee creates the purchase but approval process starts from Finance
      return 'finance'; // First approval step is Finance
    }
    
    // Check the last approval action
    const lastApproval = history[history.length - 1];
    
    // If last action was Employee submitting/creating, next step is Finance
    if (lastApproval.action === 'CREATED' || lastApproval.action === 'SUBMITTED' || 
        lastApproval.action === 'APPROVED' && lastApproval.comments?.includes('Purchase submitted by Employee')) {
      return 'finance';
    }
    
    // If Finance approved and escalated to Director
    if (lastApproval.action === 'APPROVED' && lastApproval.comments?.includes('Escalated to Director')) {
      return 'director';
    }
    
    // If we're in pending status, determine who should act next based on history
    if (isPendingApproval) {
      // Check if there's an escalation or director approval needed
      const financeApproval = history.find(h => 
        h.action === 'APPROVED' && 
        (h.comments?.includes('Escalated') || h.comments?.includes('Director'))
      );
      if (financeApproval) {
        return 'director';
      }
      
      // Check if employee step is completed
      const employeeSubmission = history.find(h => 
        h.action === 'CREATED' || h.action === 'SUBMITTED' ||
        (h.action === 'APPROVED' && h.comments?.includes('Purchase submitted by Employee'))
      );
      if (employeeSubmission) {
        return 'finance';
      }
      
      return 'finance'; // Default to finance if no clear employee submission found
    }
    
    return null; // Default fallback
  };
  
  const activeStep = getActiveApprovalStep();
  const isUserTurn = activeStep === normalizedRole;
  const canUserAct = isUserTurn && canApprove && !isApproved && !isRejected;
  
  const shouldShowDirectorCheckbox = isFinanceRole && canUserAct && (isPendingApproval || isNotStarted);
  const isDisabled = isApproved || isRejected || !canUserAct; // Disable when completed or not user's turn
  
  // Debug logging
  console.log('ApprovalPanel Debug:', {
    userRole,
    normalizedRole,
    isFinanceRole,
    canApprove,
    approvalStatus,
    isPendingApproval,
    isNotStarted,
    shouldShowDirectorCheckbox
  });

  return (
    <Box borderWidth="1px" borderRadius="md" p={4} w="full" bg={panelBg} borderColor={panelBorderColor}>
      <HStack justify="space-between" mb={2}>
        <Text fontWeight="bold" color={panelHeadingColor}>Approval</Text>
        <Badge colorScheme={statusColor(approvalStatus)}>{(approvalStatus || 'NOT_REQUIRED').replace('_',' ')}</Badge>
      </HStack>
      <Divider mb={3} borderColor={dividerColor} />
      {loading ? (
        <HStack><Spinner size="sm" /><Text color={bodyTextColor}>Loading approval history...</Text></HStack>
      ) : error ? (
        <Alert status="error" mb={2}><AlertIcon />{error}</Alert>
      ) : history.length === 0 ? (
        <Text color={mutedTextColor}>No approval history.</Text>
      ) : (
        <VStack align="stretch" spacing={2}>
          {history.map((h, i) => (
            <Box
              key={i}
              p={3}
              borderRadius="md"
              bg={h.action === 'REJECTED' ? historyRejectedBg : historyCardBg}
              borderLeft="3px solid"
              borderLeftColor={h.action === 'REJECTED' ? 'red.500' : statusColor(h.action) + '.500'}
              borderWidth="1px"
              borderColor={panelBorderColor}
            >
              <HStack justify="space-between" mb={1}>
                <Text fontSize="sm" fontWeight="medium" color={bodyTextColor}>
                  {h.user ? (
                    <>
                      <Text as="span" color={roleTextColor} fontWeight="semibold">
                        {formatRoleForApproval(h.user.role)}
                      </Text>
                      <Text as="span" color={mutedTextColor} mx={1}>
                        -
                      </Text>
                      <Text as="span">
                        {h.user.first_name} {h.user.last_name}
                      </Text>
                    </>
                  ) : (
                    'Unknown User'
                  )}
                </Text>
                <Badge colorScheme={statusColor(h.action)}>{h.action}</Badge>
              </HStack>
              <Text fontSize="xs" color={mutedTextColor} mb={1}>
                {new Date(h.created_at).toLocaleString('id-ID')}
              </Text>
              {h.comments && (
                <Text fontSize="sm" color={bodyTextColor} fontStyle={h.action === 'REJECTED' ? 'italic' : 'normal'}>
                  "{h.comments}"
                </Text>
              )}
            </Box>
          ))}
        </VStack>
      )}

      {canApprove && (
        <>
          <Divider my={3} borderColor={dividerColor} />
          <VStack spacing={3} align="stretch">
            {/* Display current role for debugging and turn status */}
            <Text fontSize="xs" color={mutedTextColor}>
              Logged in as: {userRole} (normalized: {normalizedRole})
            </Text>
            
            {/* Turn status message */}
            {!isUserTurn && activeStep && (
              <Alert status="info" size="sm" borderRadius="md">
                <AlertIcon />
                <Text fontSize="sm">
                  Menunggu persetujuan dari: <Text as="span" fontWeight="bold">{formatRoleForApproval(activeStep)}</Text>
                  {activeStep === 'director' && ' (sudah di-escalate ke Director)'}
                </Text>
              </Alert>
            )}
            
            {(isApproved || isRejected) && (
              <Alert status={isApproved ? "success" : "error"} size="sm" borderRadius="md">
                <AlertIcon />
                <Text fontSize="sm">
                  Proses approval telah <Text as="span" fontWeight="bold">{isApproved ? 'diselesaikan' : 'ditolak'}</Text>
                </Text>
              </Alert>
            )}
            {/* Enhanced validation message */}
            {getValidationMessage() && (
              <Alert 
                status={getValidationMessage()!.severity} 
                size="sm"
                borderRadius="md"
              >
                <AlertIcon />
                {getValidationMessage()!.message}
              </Alert>
            )}
            
            <FormControl>
              <FormLabel fontSize="sm" color={panelHeadingColor}>Komentar</FormLabel>
              <Textarea
                value={comments}
                onChange={handleCommentsChange}
                placeholder={hasUserInteracted ? "Contoh: Harga terlalu tinggi dibanding market rate, perlu negosiasi ulang..." : "Tambahkan catatan persetujuan/penolakan (opsional untuk Approve, wajib untuk Reject)"}
                rows={3}
                isDisabled={false} // Keep comment field always enabled
                bg={textAreaBg}
                color={bodyTextColor}
                borderColor={validationAttempted && !comments.trim() ? 'red.300' : textAreaBorderColor}
                _placeholder={{ color: mutedTextColor }}
                _hover={{
                  borderColor: validationAttempted && !comments.trim() ? 'red.400' : textAreaHoverColor
                }}
                _focus={{
                  borderColor: validationAttempted && !comments.trim() ? 'red.500' : 'blue.500',
                  boxShadow: validationAttempted && !comments.trim() 
                    ? '0 0 0 1px var(--chakra-colors-red-500)' 
                    : '0 0 0 1px var(--chakra-colors-blue-500)'
                }}
              />
              <FormHelperText fontSize="xs" color={
                validationAttempted && !comments.trim()
                  ? 'red.500'
                  : hasUserInteracted && comments.trim()
                    ? 'green.600'
                    : mutedTextColor
              }>
                {validationAttempted && !comments.trim()
                  ? 'Komentar wajib diisi untuk penolakan'
                  : hasUserInteracted && comments.trim()
                    ? `${comments.trim().length} karakter - Alasan yang baik membantu transparansi proses`
                    : 'Untuk Reject, komentar wajib diisi. Untuk Approve, komentar opsional.'}
              </FormHelperText>
            </FormControl>
            
            {/* Director approval checkbox for finance role - only disabled during explicit rejection */}
            {shouldShowDirectorCheckbox && (
              <FormControl>
                <Checkbox 
                  isChecked={requiresDirector} 
                  onChange={(e) => setRequiresDirector(e.target.checked)}
                  colorScheme="blue"
                  isDisabled={approvalIntent === 'reject'} // Only disable when user explicitly clicks reject
                >
                  <Text fontSize="sm" fontWeight="medium" color={panelHeadingColor}>
                    Butuh persetujuan Director
                  </Text>
                </Checkbox>
                <FormHelperText fontSize="xs" color={directorHelperColor}>
                  {approvalIntent === 'reject' ? (
                    <Text color="orange.600">
                      Checkbox dinonaktifkan - tidak dapat meminta persetujuan Director untuk penolakan
                    </Text>
                  ) : (
                    <>
                      Centang jika purchase ini memerlukan persetujuan tambahan dari Director
                      {purchaseAmount > 0 && (
                        <Text as="span" fontWeight="medium">
                          {' '}(Total: {new Intl.NumberFormat('id-ID', {
                            style: 'currency',
                            currency: 'IDR',
                            minimumFractionDigits: 0,
                          }).format(purchaseAmount)})
                        </Text>
                      )}
                      {comments.trim().length > 0 && (
                        <Text as="div" color={directorHintColor} fontSize="xs" mt={1}>
                          💡 Anda dapat menambahkan komentar dan tetap meminta persetujuan Director untuk approval
                        </Text>
                      )}
                    </>
                  )}
                </FormHelperText>
              </FormControl>
            )}
          </VStack>
          <HStack justify="flex-end" spacing={3} pt={2}>
            <Button
              size="sm"
              variant="outline"
              colorScheme="red"
              onClick={async () => {
                setApprovalIntent('reject'); // Set rejection intent
                setValidationAttempted(true);
                
                if (comments.trim() === '') {
                  toast({
                    title: 'Komentar Wajib',
                    description: 'Berdasarkan SOP, alasan penolakan harus dicantumkan sebelum melanjutkan proses',
                    status: 'error',
                    duration: 5000,
                    isClosable: true,
                  });
                  
                  // Auto focus ke textarea untuk UX yang lebih baik
                  setTimeout(() => {
                    const textarea = document.querySelector('textarea[placeholder*="Contoh"]');
                    if (textarea) {
                      (textarea as HTMLTextAreaElement).focus();
                      (textarea as HTMLTextAreaElement).scrollIntoView({ behavior: 'smooth', block: 'center' });
                    }
                  }, 100);
                  return;
                }
                
                if (onReject && !isDisabled) {
                  await onReject(comments);
                  // Refresh history after rejection and reset states
                  setHasUserInteracted(false);
                  setValidationAttempted(false);
                  setApprovalIntent(null);
                  setTimeout(() => refresh(), 500);
                }
              }}
              isLoading={actionLoading}
              isDisabled={isDisabled} // Disable based on turn logic
              title={
                isApproved || isRejected
                  ? 'Proses approval sudah selesai'
                  : !isUserTurn
                    ? `Menunggu giliran approval dari ${activeStep}`
                    : 'Tolak purchase ini'
              }
              opacity={getButtonOpacity()}
              transition="all 0.3s ease"
              _hover={{
                opacity: 1,
                transform: 'translateY(-1px)',
                boxShadow: rejectHoverShadow
              }}
              _active={{
                transform: 'translateY(0px)'
              }}
            >
              Tolak
            </Button>
            <Button
              size="sm"
              colorScheme="green"
              onClick={async () => {
                if (onApprove && !isDisabled) {
                  await onApprove(comments, requiresDirector);
                  // Refresh history after approval and reset states
                  setHasUserInteracted(false);
                  setValidationAttempted(false);
                  setTimeout(() => refresh(), 500);
                }
              }}
              isLoading={actionLoading}
              isDisabled={isDisabled} // Disable based on turn logic
              title={
                isApproved || isRejected
                  ? 'Proses approval sudah selesai'
                  : !isUserTurn
                    ? `Menunggu giliran approval dari ${activeStep}`
                    : 'Setujui purchase ini'
              }
              transition="all 0.3s ease"
              _hover={{
                transform: 'translateY(-1px)',
                boxShadow: approveHoverShadow
              }}
              _active={{
                transform: 'translateY(0px)'
              }}
            >
              Setujui
            </Button>
          </HStack>
        </>
      )}
    </Box>
  );
};
