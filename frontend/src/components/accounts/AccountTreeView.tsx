'use client';

import React from 'react';
import {
  Box,
  Flex,
  Text,
  Icon,
  Badge,
  useDisclosure,
  Button,
  HStack,
  VStack,
} from '@chakra-ui/react';
import { 
  FiChevronRight, 
  FiChevronDown, 
  FiEdit, 
  FiTrash2,
  FiFolder,
  FiFile 
} from 'react-icons/fi';
import { Account } from '@/types/account';
import { useAuth } from '@/contexts/AuthContext';
import { useTranslation } from '@/hooks/useTranslation';
import accountService from '@/services/accountService';
import { shouldEmphasizeAccountRow } from '@/utils/accountDisplay';

interface AccountTreeViewProps {
  accounts: Account[];
  onEdit?: (account: Account) => void;
  onDelete?: (account: Account) => void;
  onAdminDelete?: (account: Account) => void; // Admin-only delete for header accounts
  showActions?: boolean;
  showBalance?: boolean;
}

interface TreeNodeProps {
  account: Account;
  level: number;
  onEdit?: (account: Account) => void;
  onDelete?: (account: Account) => void;
  onAdminDelete?: (account: Account) => void;
  showActions?: boolean;
  showBalance?: boolean;
}

const TreeNode: React.FC<TreeNodeProps> = ({
  account,
  level,
  onEdit,
  onDelete,
  onAdminDelete,
  showActions = true,
  showBalance = true,
}) => {
  const { user } = useAuth();
  const { t } = useTranslation();
  const isAdmin = user?.role?.toLowerCase() === 'admin';
  const { isOpen, onToggle } = useDisclosure();
  const hasChildren = account.children && account.children.length > 0;
  const childCount = account.child_count ?? account.children?.length ?? 0;
  const indentWidth = level * 20;
  const isEmphasized = shouldEmphasizeAccountRow(account);

  const handleEdit = (e: React.MouseEvent) => {
    e.stopPropagation();
    onEdit?.(account);
  };

  const handleDelete = (e: React.MouseEvent) => {
    e.stopPropagation();
    onDelete?.(account);
  };

  const handleAdminDelete = (e: React.MouseEvent) => {
    e.stopPropagation();
    onAdminDelete?.(account);
  };

  return (
    <Box>
      <Flex
        align="center"
        p={2}
        pl={`${indentWidth + 8}px`}
        _hover={{ bg: 'gray.50' }}
        cursor={hasChildren ? 'pointer' : 'default'}
        onClick={hasChildren ? onToggle : undefined}
        borderRadius="md"
      >
        {/* Expand/Collapse Icon */}
        <Box w="20px" mr={2}>
          {hasChildren ? (
            <Icon
              as={isOpen ? FiChevronDown : FiChevronRight}
              color="gray.500"
            />
          ) : null}
        </Box>

        {/* Account Icon */}
        <Icon
          as={account.is_header ? FiFolder : FiFile}
          color={account.is_active ? accountService.getAccountTypeColor(account.type) : 'gray.400'}
          mr={2}
        />

        {/* Account Info */}
        <Flex flex={1} align="center" justify="space-between">
          <HStack spacing={3}>
            <Text
              fontWeight={isEmphasized ? 'bold' : 'normal'}
              color={account.is_active ? 'gray.800' : 'gray.400'}
              fontSize={account.level === 1 ? 'md' : 'sm'}
            >
              {account.code} - {account.name}
            </Text>
            
            <Badge
              colorScheme={accountService.getAccountTypeColor(account.type)}
              size="sm"
              variant="subtle"
            >
              {t(`accounts.types.${account.type.toLowerCase()}`)}
            </Badge>

            {!account.is_active && (
              <Badge colorScheme="gray" size="sm">
                {t('accounts.inactive')}
              </Badge>
            )}
            
            {/* Show child count for parent accounts */}
            {account.is_header && childCount > 0 && (
              <Badge colorScheme="blue" size="sm" variant="solid">
                {childCount} {childCount !== 1 ? t('accounts.children') : t('accounts.child')}
              </Badge>
            )}
          </HStack>

          <HStack spacing={3}>
            {showBalance && (
              <VStack spacing={0} align="end">
                <Text
                  fontSize="sm"
                  fontWeight={isEmphasized ? "bold" : "medium"}
                  color="green.600"
                >
                  {accountService.formatBalance(account.balance, 'IDR', account.code, account.type)}
                </Text>
              </VStack>
            )}

            {/* Regular accounts actions */}
            {showActions && !account.is_header && (
              <HStack spacing={1}>
                <Button
                  size="xs"
                  variant="ghost"
                  leftIcon={<FiEdit />}
                  onClick={handleEdit}
                  isDisabled={!account.is_active}
                >
                  {t('common.edit')}
                </Button>
                <Button
                  size="xs"
                  variant="ghost"
                  colorScheme="red"
                  leftIcon={<FiTrash2 />}
                  onClick={handleDelete}
                  isDisabled={!account.is_active || hasChildren}
                >
                  {t('common.delete')}
                </Button>
              </HStack>
            )}
            
            {/* Header accounts - admin only delete */}
            {showActions && account.is_header && isAdmin && onAdminDelete && (
              <HStack spacing={1}>
                <Button
                  size="xs"
                  variant="ghost"
                  colorScheme="red"
                  leftIcon={<FiTrash2 />}
                  onClick={handleAdminDelete}
                >
                  {t('common.delete')}
                </Button>
              </HStack>
            )}
          </HStack>
        </Flex>
      </Flex>

      {/* Children */}
      {hasChildren && isOpen ? (
          <Box>
            {account.children?.map((child) => (
              <TreeNode
                key={child.id}
                account={child}
                level={level + 1}
                onEdit={onEdit}
                onDelete={onDelete}
                onAdminDelete={onAdminDelete}
                showActions={showActions}
                showBalance={showBalance}
              />
            ))}
          </Box>
        ) : null}
    </Box>
  );
};

const AccountTreeView: React.FC<AccountTreeViewProps> = ({
  accounts,
  onEdit,
  onDelete,
  onAdminDelete,
  showActions = true,
  showBalance = true,
}) => {
  const { t } = useTranslation();
  
  if (accounts.length === 0) {
    return (
      <Box p={4} textAlign="center" color="gray.500">
        <Text>{t('accounts.noAccountsFound')}</Text>
      </Box>
    );
  }

  return (
    <Box>
      {accounts.map((account) => (
        <TreeNode
          key={account.id}
          account={account}
          level={0}
          onEdit={onEdit}
          onDelete={onDelete}
          onAdminDelete={onAdminDelete}
          showActions={showActions}
          showBalance={showBalance}
        />
      ))}
    </Box>
  );
};

export default AccountTreeView;
