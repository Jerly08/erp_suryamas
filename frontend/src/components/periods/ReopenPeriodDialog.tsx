import React, { useEffect, useState } from 'react';
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  Button,
  FormControl,
  FormLabel,
  Textarea,
  Text,
  Alert,
  AlertIcon,
  AlertDescription,
  VStack,
  HStack,
  Box,
  Icon,
} from '@chakra-ui/react';
import { FiCalendar, FiUnlock, FiAlertCircle } from 'react-icons/fi';
import api from '@/services/api';
import { useToast } from '@chakra-ui/react';
import { formatPeriodRange } from '@/services/periodClosingService';

interface ReopenPeriodDialogProps {
  isOpen: boolean;
  onClose: () => void;
  startDate: string;
  endDate: string;
  onSuccess?: () => void;
}

export const ReopenPeriodDialog: React.FC<ReopenPeriodDialogProps> = ({
  isOpen,
  onClose,
  startDate,
  endDate,
  onSuccess,
}) => {
  const [reason, setReason] = useState('');
  const [loading, setLoading] = useState(false);
  const toast = useToast();

  useEffect(() => {
    if (!isOpen) {
      setReason('');
    }
  }, [isOpen]);

  const handleReopen = async () => {
    if (!reason.trim()) {
      toast({
        title: 'Reason Required',
        description: 'Please provide a reason for reopening this period',
        status: 'warning',
        duration: 3000,
      });
      return;
    }

    setLoading(true);
    try {
      const response = await api.post('/api/v1/period-closing/reopen', {
        start_date: startDate,
        end_date: endDate,
        reason: reason.trim(),
      });

      if (response.data.success) {
        toast({
          title: 'Period Reopened Successfully',
          description: `Period ${startDate} to ${endDate} has been reopened`,
          status: 'success',
          duration: 5000,
        });
        
        if (onSuccess) {
          onSuccess();
        }
        
        onClose();
      }
    } catch (error: unknown) {
      const apiError = error as {
        response?: { data?: { details?: string; error?: string } };
      };
      const errorMessage = apiError.response?.data?.details ||
        apiError.response?.data?.error ||
        'Failed to reopen period';
      
      toast({
        title: 'Failed to Reopen Period',
        description: errorMessage,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="lg">
      <ModalOverlay />
      <ModalContent>
        <ModalHeader>
          <HStack spacing={2}>
            <Icon as={FiUnlock} color="orange.500" />
            <Text>Reopen Closed Period</Text>
          </HStack>
        </ModalHeader>
        <ModalCloseButton />
        
        <ModalBody>
          <VStack spacing={4} alignItems="stretch">
            <Alert status="warning" borderRadius="md">
              <AlertIcon />
              <Box>
                <AlertDescription>
                  <strong>Warning:</strong> Reopening this period will:
                  <ul style={{ marginTop: '8px', paddingLeft: '20px' }}>
                    <li>Reverse all closing journal entries</li>
                    <li>Restore revenue and expense account balances</li>
                    <li>Allow new transactions in this period</li>
                  </ul>
                </AlertDescription>
              </Box>
            </Alert>

            <Box p={3} bg="gray.50" borderRadius="md">
              <HStack spacing={2} mb={2}>
                <Icon as={FiCalendar} color="gray.600" />
                <Text fontWeight="semibold">Period to Reopen:</Text>
              </HStack>
              <Text fontSize="lg" color="blue.600">
                {formatPeriodRange(startDate, endDate)}
              </Text>
            </Box>

            <FormControl isRequired>
              <FormLabel>Reason for Reopening</FormLabel>
              <Textarea
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="Explain why this period needs to be reopened (e.g., correction needed, missing transactions, etc.)"
                rows={4}
                resize="vertical"
              />
            </FormControl>

            <Alert status="info" size="sm" borderRadius="md">
              <AlertIcon />
              <AlertDescription fontSize="sm">
                <Icon as={FiAlertCircle} /> Note: You cannot reopen a period if newer periods have been closed after it.
              </AlertDescription>
            </Alert>
          </VStack>
        </ModalBody>

        <ModalFooter>
          <Button variant="ghost" mr={3} onClick={onClose} isDisabled={loading}>
            Cancel
          </Button>
          <Button
            colorScheme="orange"
            onClick={handleReopen}
            isLoading={loading}
            loadingText="Reopening..."
            isDisabled={!reason.trim() || loading}
          >
            Reopen Period
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};
