'use client';

import React from 'react';
import {
  Box,
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
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Icon,
  useColorModeValue,
} from '@chakra-ui/react';
import { FiClock, FiLogIn, FiShield } from 'react-icons/fi';
import { useTranslation } from '@/hooks/useTranslation';

interface AuthExpiredModalProps {
  isOpen: boolean;
  onLoginRedirect: () => void;
}

const AuthExpiredModal: React.FC<AuthExpiredModalProps> = ({
  isOpen,
  onLoginRedirect,
}) => {
  const { t } = useTranslation();
  const bgColor = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const headerBg = useColorModeValue('blue.500', 'blue.400');
  const iconBadgeBg = useColorModeValue('whiteAlpha.300', 'whiteAlpha.200');
  const infoBg = useColorModeValue('blue.50', 'whiteAlpha.100');
  const infoBorderColor = useColorModeValue('blue.200', 'blue.300');
  const mutedTextColor = useColorModeValue('gray.600', 'gray.300');
  const causesBg = useColorModeValue('gray.50', 'whiteAlpha.50');
  const emphasisTextColor = useColorModeValue('gray.700', 'gray.100');
  const [isRedirecting, setIsRedirecting] = React.useState(false);
  
  // Debug effect to track modal state
  React.useEffect(() => {
    console.log('AuthExpiredModal - isOpen changed:', isOpen);
  }, [isOpen]);
  
  // Remove legacy browser tokens without marking an explicit logout. A stale
  // response from another tab must not invalidate a newly-created cookie session.
  React.useEffect(() => {
    if (isOpen && typeof window !== 'undefined') {
      window.localStorage.removeItem('token');
      window.localStorage.removeItem('refreshToken');
      window.localStorage.removeItem('user');
      console.log('AuthExpiredModal - Cleared legacy localStorage auth data');
    }
  }, [isOpen]);
  
  const handleLoginRedirect = () => {
    if (isRedirecting) return; // Prevent double-clicks
    
    setIsRedirecting(true);
    console.log('AuthExpiredModal - Redirecting to login');
    
    // Small delay to show loading state
    setTimeout(() => {
      onLoginRedirect();
    }, 500);
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={() => {}} // Prevent closing without action
      closeOnOverlayClick={false}
      closeOnEsc={false}
      size="md"
      isCentered
    >
      <ModalOverlay bg="blackAlpha.600" backdropFilter="blur(5px)" />
      <ModalContent
        bg={bgColor}
        borderRadius="xl"
        boxShadow="xl"
        border="1px solid"
        borderColor={borderColor}
        mx={4}
      >
        <ModalHeader
          bg={headerBg}
          color="white"
          borderTopRadius="xl"
          textAlign="center"
          py={5}
        >
          <VStack spacing={2}>
            <Box
              display="inline-flex"
              alignItems="center"
              justifyContent="center"
              w="12"
              h="12"
              borderRadius="full"
              bg={iconBadgeBg}
            >
              <Icon as={FiClock} boxSize={5} />
            </Box>
            <Text fontSize="lg" fontWeight="bold">
              {t('auth.session.expiredTitle')}
            </Text>
          </VStack>
        </ModalHeader>

        <ModalBody py={6}>
          <VStack spacing={5} align="stretch">
            <Alert
              status="info"
              variant="left-accent"
              borderRadius="lg"
              bg={infoBg}
              border="1px solid"
              borderColor={infoBorderColor}
              alignItems="flex-start"
            >
              <AlertIcon />
              <VStack align="start" spacing={1} flex={1}>
                <AlertTitle fontSize="sm">
                  {t('auth.session.authRequired')}
                </AlertTitle>
                <AlertDescription fontSize="sm">
                  {t('auth.session.expiredMessage')}
                </AlertDescription>
              </VStack>
            </Alert>

            <HStack
              align="start"
              spacing={3}
              p={4}
              borderRadius="lg"
              bg={causesBg}
            >
              <Box
                display="inline-flex"
                alignItems="center"
                justifyContent="center"
                w="9"
                h="9"
                borderRadius="full"
                bg={infoBg}
                color="blue.500"
                flexShrink={0}
              >
                <Icon as={FiShield} boxSize={4} />
              </Box>
              <VStack align="start" spacing={2} flex={1}>
                <Text fontSize="sm" fontWeight="semibold" color={emphasisTextColor}>
                  {t('auth.session.securityNote')}
                </Text>
                <Text fontSize="xs" color={mutedTextColor}>
                  {t('auth.session.possibleCauses')}
                </Text>
                <VStack align="start" spacing={1} fontSize="sm" color={mutedTextColor}>
                  <Text>{t('auth.session.tokenExpired')}</Text>
                  <Text>{t('auth.session.otherDevice')}</Text>
                  <Text>{t('auth.session.timeout')}</Text>
                </VStack>
              </VStack>
            </HStack>

            <Text fontSize="sm" color={mutedTextColor} textAlign="center">
              {t('auth.session.protectionNote')}
            </Text>
          </VStack>
        </ModalBody>

        <ModalFooter justifyContent="center" pb={6}>
          <Button
            colorScheme="blue"
            leftIcon={isRedirecting ? undefined : <FiLogIn />}
            onClick={handleLoginRedirect}
            size="lg"
            minW="200px"
            borderRadius="lg"
            isLoading={isRedirecting}
            loadingText={t('auth.session.redirecting')}
            disabled={isRedirecting}
          >
            {isRedirecting ? t('auth.session.redirecting') : t('auth.session.relogin')}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export default AuthExpiredModal;
