'use client';

import React, { useState } from 'react';
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  Button,
  Text,
  VStack,
  HStack,
  RadioGroup,
  Radio,
  Select,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Box,
  Divider,
  Badge,
  Icon,
} from '@chakra-ui/react';
import { FiAlertTriangle, FiTrash2, FiMove, FiArrowDown } from 'react-icons/fi';
import { Account } from '@/types/account';

interface AdminDeleteDialogProps {
  isOpen: boolean;
  onClose: () => void;
  account: Account | null;
  parentAccounts: Account[];
  childAccounts: Account[];
  onConfirm: (options: { cascade_delete: boolean; new_parent_id?: number }) => void;
  isSubmitting?: boolean;
}

const AdminDeleteDialog: React.FC<AdminDeleteDialogProps> = ({
  isOpen,
  onClose,
  account,
  parentAccounts,
  childAccounts,
  onConfirm,
  isSubmitting = false,
}) => {
  const [deleteOption, setDeleteOption] = useState<'cascade' | 'transfer' | 'root'>('root');
  const [selectedParentId, setSelectedParentId] = useState<number | null>(null);

  const handleConfirm = () => {
    if (deleteOption === 'cascade') {
      onConfirm({ cascade_delete: true });
    } else if (deleteOption === 'transfer' && selectedParentId) {
      onConfirm({ cascade_delete: false, new_parent_id: selectedParentId });
    } else {
      onConfirm({ cascade_delete: false });
    }
  };

  const handleClose = () => {
    if (!isSubmitting) {
      setDeleteOption('root');
      setSelectedParentId(null);
      onClose();
    }
  };

  if (!account) return null;

  const hasChildren = childAccounts && childAccounts.length > 0;

  return (
    <Modal isOpen={isOpen} onClose={handleClose} size="lg" closeOnOverlayClick={!isSubmitting}>
      <ModalOverlay />
      <ModalContent>
        <ModalHeader>
          <HStack>
            <Icon as={FiAlertTriangle} color="red.500" />
            <Text>Delete Header Account</Text>
          </HStack>
        </ModalHeader>

        <ModalBody>
          <VStack spacing={4} align="stretch">
            <Alert status="warning" borderRadius="md">
              <AlertIcon />
              <Box>
                <AlertTitle fontSize="sm">Admin Delete Confirmation</AlertTitle>
                <AlertDescription fontSize="xs">
                  You are about to delete a header account. This action requires admin privileges.
                </AlertDescription>
              </Box>
            </Alert>

            <Box>
              <Text fontWeight="medium" fontSize="sm" mb={2}>Account to Delete:</Text>
              <Box p={3} bg="gray.50" borderRadius="md" border="1px" borderColor="gray.200">
                <HStack>
                  <Badge colorScheme="blue">HEADER</Badge>
                  <Text fontWeight="medium">{account.code} - {account.name}</Text>
                </HStack>
                <Text fontSize="xs" color="gray.600" mt={1}>
                  Type: {account.type} • Balance: {account.balance}
                </Text>
              </Box>
            </Box>

            {hasChildren && (
              <>
                <Box>
                  <Text fontWeight="medium" fontSize="sm" mb={2}>
                    Child Accounts ({childAccounts.length}):
                  </Text>
                  <Box 
                    maxH="150px" 
                    overflowY="auto" 
                    p={3} 
                    bg="gray.50" 
                    borderRadius="md" 
                    border="1px" 
                    borderColor="gray.200"
                  >
                    <VStack spacing={1} align="stretch">
                      {childAccounts.map((child) => (
                        <Box key={child.id} fontSize="xs">
                          <Text fontWeight="medium">{child.code} - {child.name}</Text>
                          <Text color="gray.600">Balance: {child.balance}</Text>
                        </Box>
                      ))}
                    </VStack>
                  </Box>
                </Box>

                <Divider />

                <Box>
                  <Text fontWeight="medium" fontSize="sm" mb={3}>
                    What should happen to child accounts?
                  </Text>
                  <RadioGroup
                    value={deleteOption}
                    onChange={(value) => setDeleteOption(value as 'cascade' | 'transfer' | 'root')}
                  >
                    <VStack spacing={3} align="stretch">
                      <Radio value="root">
                        <VStack align="start" spacing={1}>
                          <HStack>
                            <Icon as={FiArrowDown} color="blue.500" />
                            <Text fontSize="sm" fontWeight="medium">Move to Root Level</Text>
                          </HStack>
                          <Text fontSize="xs" color="gray.600">
                            Child accounts will become top-level accounts (no parent)
                          </Text>
                        </VStack>
                      </Radio>

                      <Radio value="transfer">
                        <VStack align="start" spacing={1}>
                          <HStack>
                            <Icon as={FiMove} color="green.500" />
                            <Text fontSize="sm" fontWeight="medium">Transfer to Another Parent</Text>
                          </HStack>
                          <Text fontSize="xs" color="gray.600">
                            Move child accounts to a different parent account
                          </Text>
                        </VStack>
                      </Radio>

                      {deleteOption === 'transfer' && (
                        <Box ml={6}>
                          <Text fontSize="xs" mb={2}>Select new parent:</Text>
                          <Select
                            placeholder="Choose parent account..."
                            size="sm"
                            value={selectedParentId || ''}
                            onChange={(e) => setSelectedParentId(e.target.value ? parseInt(e.target.value) : null)}
                          >
                            {parentAccounts
                              .filter(parent => 
                                parent.id !== account.id && // Don't show self
                                parent.type === account.type && // Same type
                                parent.is_header // Only header accounts as parents
                              )
                              .map(parent => (
                                <option key={parent.id} value={parent.id}>
                                  {parent.code} - {parent.name}
                                </option>
                              ))}
                          </Select>
                        </Box>
                      )}

                      <Radio value="cascade">
                        <VStack align="start" spacing={1}>
                          <HStack>
                            <Icon as={FiTrash2} color="red.500" />
                            <Text fontSize="sm" fontWeight="medium" color="red.500">
                              Delete All (Cascade Delete)
                            </Text>
                          </HStack>
                          <Text fontSize="xs" color="red.600">
                            ⚠️ Delete this account AND all child accounts permanently
                          </Text>
                        </VStack>
                      </Radio>
                    </VStack>
                  </RadioGroup>
                </Box>

                {deleteOption === 'cascade' && (
                  <Alert status="error" borderRadius="md">
                    <AlertIcon />
                    <Box>
                      <AlertTitle fontSize="sm">Cascade Delete Warning</AlertTitle>
                      <AlertDescription fontSize="xs">
                        This will permanently delete {childAccounts.length + 1} accounts.
                        This action cannot be undone.
                      </AlertDescription>
                    </Box>
                  </Alert>
                )}
              </>
            )}

            {!hasChildren && (
              <Alert status="info" borderRadius="md">
                <AlertIcon />
                <Text fontSize="sm">
                  This header account has no child accounts. It will be deleted directly.
                </Text>
              </Alert>
            )}
          </VStack>
        </ModalBody>

        <ModalFooter>
          <HStack spacing={3}>
            <Button 
              variant="outline" 
              onClick={handleClose}
              isDisabled={isSubmitting}
            >
              Cancel
            </Button>
            <Button
              colorScheme="red"
              onClick={handleConfirm}
              isLoading={isSubmitting}
              isDisabled={deleteOption === 'transfer' && !selectedParentId}
            >
              {deleteOption === 'cascade' 
                ? `Delete All (${hasChildren ? childAccounts.length + 1 : 1})`
                : 'Delete Account'
              }
            </Button>
          </HStack>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export default AdminDeleteDialog;
