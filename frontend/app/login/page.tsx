'use client';

import React, { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/contexts/AuthContext';
import { useBranding } from '@/contexts/BrandingContext';
import { getScaledLogoSize } from '@/config/branding';
import SimpleLayout from '@/components/layout/SimpleLayout';
import {
  Box,
  Flex,
  FormControl,
  FormLabel,
  Input,
  Button,
  Text,
  Heading,
  useToast,
  InputGroup,
  InputRightElement,
  IconButton,
  Image,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  VStack,
  HStack,
  Divider,
  useColorModeValue,
  Card,
  CardBody,
  Container,
  SimpleGrid,
  Badge,
  Center
} from '@chakra-ui/react';
import { FiLogIn, FiEye, FiEyeOff, FiShield, FiLock, FiMail, FiUsers, FiTrendingUp } from 'react-icons/fi';

const LoginContent = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  
  const { login, isAuthenticated } = useAuth();
  const { companyName, logoUrl, logoScale, poweredByText } = useBranding();
  const router = useRouter();
  const toast = useToast();
  
  // Color mode values for theming
  const bgGradient = useColorModeValue(
    'linear(to-br, blue.50, purple.50, pink.50)',
    'linear(to-br, gray.900, blue.900, purple.900)'
  );
  const cardBg = useColorModeValue('white', 'gray.800');
  const headingColor = useColorModeValue('gray.800', 'white');
  const textColor = useColorModeValue('gray.600', 'gray.300');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const inputBg = useColorModeValue('white', 'gray.700');
  const featureCardBg = useColorModeValue('white', 'gray.700');
  const iconColor = useColorModeValue('blue.500', 'blue.300');
  const accentColor = useColorModeValue('blue.500', 'blue.400');
  const inputHoverBorderColor = useColorModeValue('blue.300', 'blue.500');
  const inputPlaceholderColor = useColorModeValue('gray.400', 'gray.500');
  const buttonHoverBg = useColorModeValue('blue.600', 'blue.300');
  const decorativeElementBg1 = useColorModeValue('blue.100', 'blue.800');
  const decorativeElementBg2 = useColorModeValue('purple.100', 'purple.800');
  const passwordToggleHoverBg = useColorModeValue('gray.100', 'gray.600');
  const supportPanelBg = useColorModeValue('blue.50', 'whiteAlpha.100');
  const supportPanelBorder = useColorModeValue('blue.100', 'whiteAlpha.200');
  const supportIconBg = useColorModeValue('white', 'whiteAlpha.200');
  // Error alert colors
  const errorAlertBg = useColorModeValue('red.50', 'red.900');
  const errorAlertBorderColor = useColorModeValue('red.200', 'red.700');
  
  useEffect(() => {
    if (isAuthenticated) {
      router.replace('/dashboard');
    }
  }, [isAuthenticated, router]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    
    if (!email || !password) {
      setError('Please enter both email and password');
      return;
    }
    
    try {
      setIsSubmitting(true);
      await login(email, password);
      
      // Clear logout timestamp on successful login
      if (typeof window !== 'undefined') {
        window.localStorage.removeItem('lastLogoutTime');
      }
      
      toast({
        title: 'Login Successful',
        description: "Welcome back!",
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
      
    } catch (err) {
      // Use the actual error message from the caught exception
      const errorMessage = err instanceof Error ? err.message : 'Invalid email or password';
      setError(errorMessage);
      
      toast({
        title: 'Login Failed',
        description: errorMessage,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Box 
      minH="100vh"
      bgGradient={bgGradient}
      position="relative"
      overflow="hidden"
    >
      {/* Background decorative elements */}
      <Box
        position="absolute"
        top="-50px"
        right="-50px"
        w="200px"
        h="200px"
        bg={decorativeElementBg1}
        borderRadius="full"
        opacity={0.3}
      />
      <Box
        position="absolute"
        bottom="-100px"
        left="-100px"
        w="300px"
        h="300px"
        bg={decorativeElementBg2}
        borderRadius="full"
        opacity={0.2}
      />
      
      <Container maxW="7xl" py={8}>
        <SimpleGrid columns={{ base: 1, lg: 2 }} spacing={8} alignItems="center" minH="80vh">
          {/* Left side - Branding and Features */}
          <Box>
            <VStack spacing={8} align="stretch">
              {/* Company Branding */}
              <Box textAlign={{ base: 'center', lg: 'left' }}>
                <VStack spacing={3} align={{ base: 'center', lg: 'flex-start' }} mb={6}>
                  <Image
                    src={logoUrl}
                    alt={companyName}
                    h={{
                      base: getScaledLogoSize(56, logoScale),
                      md: getScaledLogoSize(72, logoScale),
                    }}
                    maxW={{ base: '260px', md: '360px' }}
                    objectFit="contain"
                  />
                  <VStack spacing={0} align={{ base: 'center', lg: 'flex-start' }}>
                    <Text color={textColor} fontSize="sm" fontWeight="medium">
                      Comprehensive Accounting Solution
                    </Text>
                    <Text color={textColor} fontSize="xs" opacity={0.75}>
                      {poweredByText}
                    </Text>
                  </VStack>
                </VStack>
                
                <Text 
                  fontSize="lg" 
                  color={textColor} 
                  mb={8}
                  lineHeight="tall"
                >
                  Manage your business finances with confidence. 
                  Our advanced accounting system provides comprehensive 
                  financial management tools for modern businesses.
                </Text>
              </Box>
              
              {/* Feature highlights */}
              <VStack spacing={4} align="stretch">
                <Heading size="md" color={headingColor} mb={2}>
                  Key Features
                </Heading>
                
                <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                  <Card 
                    bg={featureCardBg} 
                    shadow="sm" 
                    borderWidth="1px" 
                    borderColor={borderColor}
                    _hover={{ transform: 'translateY(-2px)', shadow: 'md' }}
                    transition="all 0.2s"
                  >
                    <CardBody p={4}>
                      <HStack spacing={3}>
                        <Center 
                          w={10} 
                          h={10} 
                          bg={useColorModeValue('blue.50', 'blue.900')} 
                          borderRadius="lg"
                        >
                          <FiShield color={iconColor} size={20} />
                        </Center>
                        <VStack align="start" spacing={0}>
                          <Text fontWeight="semibold" color={headingColor} fontSize="sm">
                            Secure Access
                          </Text>
                          <Text color={textColor} fontSize="xs">
                            Role-based permissions
                          </Text>
                        </VStack>
                      </HStack>
                    </CardBody>
                  </Card>
                  
                  <Card 
                    bg={featureCardBg} 
                    shadow="sm" 
                    borderWidth="1px" 
                    borderColor={borderColor}
                    _hover={{ transform: 'translateY(-2px)', shadow: 'md' }}
                    transition="all 0.2s"
                  >
                    <CardBody p={4}>
                      <HStack spacing={3}>
                        <Center 
                          w={10} 
                          h={10} 
                          bg={useColorModeValue('green.50', 'green.900')} 
                          borderRadius="lg"
                        >
                          <FiTrendingUp color={iconColor} size={20} />
                        </Center>
                        <VStack align="start" spacing={0}>
                          <Text fontWeight="semibold" color={headingColor} fontSize="sm">
                            Real-time Analytics
                          </Text>
                          <Text color={textColor} fontSize="xs">
                            Track performance
                          </Text>
                        </VStack>
                      </HStack>
                    </CardBody>
                  </Card>
                  
                  <Card 
                    bg={featureCardBg} 
                    shadow="sm" 
                    borderWidth="1px" 
                    borderColor={borderColor}
                    _hover={{ transform: 'translateY(-2px)', shadow: 'md' }}
                    transition="all 0.2s"
                  >
                    <CardBody p={4}>
                      <HStack spacing={3}>
                        <Center 
                          w={10} 
                          h={10} 
                          bg={useColorModeValue('purple.50', 'purple.900')} 
                          borderRadius="lg"
                        >
                          <FiUsers color={iconColor} size={20} />
                        </Center>
                        <VStack align="start" spacing={0}>
                          <Text fontWeight="semibold" color={headingColor} fontSize="sm">
                            Multi-user Support
                          </Text>
                          <Text color={textColor} fontSize="xs">
                            Collaborative workflow
                          </Text>
                        </VStack>
                      </HStack>
                    </CardBody>
                  </Card>
                  
                  <Card 
                    bg={featureCardBg} 
                    shadow="sm" 
                    borderWidth="1px" 
                    borderColor={borderColor}
                    _hover={{ transform: 'translateY(-2px)', shadow: 'md' }}
                    transition="all 0.2s"
                  >
                    <CardBody p={4}>
                      <HStack spacing={3}>
                        <Center 
                          w={10} 
                          h={10} 
                          bg={useColorModeValue('orange.50', 'orange.900')} 
                          borderRadius="lg"
                        >
                          <FiLock color={iconColor} size={20} />
                        </Center>
                        <VStack align="start" spacing={0}>
                          <Text fontWeight="semibold" color={headingColor} fontSize="sm">
                            Approval Workflow
                          </Text>
                          <Text color={textColor} fontSize="xs">
                            Automated approval
                          </Text>
                        </VStack>
                      </HStack>
                    </CardBody>
                  </Card>
                </SimpleGrid>
                
                {/* Trust badges */}
                <Box pt={6}>
                  <HStack spacing={4} justify={{ base: 'center', lg: 'flex-start' }} wrap="wrap">
                    <Badge 
                      colorScheme="green" 
                      px={3} 
                      py={1} 
                      borderRadius="full"
                      fontSize="xs"
                      fontWeight="semibold"
                    >
                      ✓ Secure
                    </Badge>
                    <Badge 
                      colorScheme="blue" 
                      px={3} 
                      py={1} 
                      borderRadius="full"
                      fontSize="xs"
                      fontWeight="semibold"
                    >
                      ✓ Fast
                    </Badge>
                    <Badge 
                      colorScheme="purple" 
                      px={3} 
                      py={1} 
                      borderRadius="full"
                      fontSize="xs"
                      fontWeight="semibold"
                    >
                      ✓ Reliable
                    </Badge>
                  </HStack>
                </Box>
              </VStack>
            </VStack>
          </Box>
          
          {/* Right side - Login Form */}
          <Flex align="center" justify="center" py={8}>
            <Box>
              <Card 
                maxW="md"
                w="full"
                bg={cardBg}
                shadow="2xl"
                borderWidth="1px"
                borderColor={borderColor}
                borderRadius="2xl"
                overflow="hidden"
              >
                <CardBody p={8}>
                  {/* Header */}
                  <VStack spacing={6} mb={8}>
                    <Image
                      src={logoUrl}
                      alt={companyName}
                      h={getScaledLogoSize(52, logoScale)}
                      maxW="220px"
                      objectFit="contain"
                    />
                    <Text color={textColor} fontSize="xs" opacity={0.75} textAlign="center">
                      {poweredByText}
                    </Text>
                    
                    <VStack spacing={2}>
                      <Heading 
                        as="h1" 
                        size="xl" 
                        textAlign="center" 
                        color={headingColor}
                        fontWeight="bold"
                        letterSpacing="tight"
                      >
                        Welcome Back
                      </Heading>
                      <Text 
                        color={textColor} 
                        textAlign="center" 
                        fontSize="md"
                        fontWeight="medium"
                      >
                        Sign in to access your accounting dashboard
                      </Text>
                    </VStack>
                  </VStack>
                  
                  {/* Error Alert */}
                  {error && (
                    <Alert 
                      status="error" 
                      mb={6} 
                      borderRadius="xl"
                      bg={errorAlertBg}
                      borderWidth="1px"
                      borderColor={errorAlertBorderColor}
                    >
                      <AlertIcon />
                      <Box>
                        <AlertTitle fontSize="sm">Login Failed!</AlertTitle>
                        <AlertDescription fontSize="sm">{error}</AlertDescription>
                      </Box>
                    </Alert>
                  )}
                  
                  {/* Login Form */}
                  <form onSubmit={handleSubmit}>
                    <VStack spacing={6}>
                      <FormControl id="email" isRequired>
                        <FormLabel 
                          color={headingColor}
                          fontSize="sm"
                          fontWeight="semibold"
                          mb={2}
                        >
                          Email address *
                        </FormLabel>
                        <InputGroup>
                          <Input
                            type="email"
                            placeholder="your-email@example.com"
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                            disabled={isSubmitting}
                            bg={inputBg}
                            borderWidth="2px"
                            borderColor={borderColor}
                            borderRadius="xl"
                            py={6}
                            fontSize="md"
                            _hover={{
                              borderColor: inputHoverBorderColor
                            }}
                            _focus={{
                              borderColor: accentColor,
                              boxShadow: `0 0 0 1px ${accentColor}`,
                              bg: inputBg
                            }}
                            _placeholder={{
                              color: inputPlaceholderColor
                            }}
                          />
                          <InputRightElement top={2} right={2}>
                            <Center 
                              w={8} 
                              h={8} 
                              bg={passwordToggleHoverBg}
                              borderRadius="lg"
                            >
                              <FiMail color={iconColor} size={16} />
                            </Center>
                          </InputRightElement>
                        </InputGroup>
                      </FormControl>
                      
                      <FormControl id="password" isRequired>
                        <FormLabel 
                          color={headingColor}
                          fontSize="sm"
                          fontWeight="semibold"
                          mb={2}
                        >
                          Password *
                        </FormLabel>
                        <InputGroup>
                          <Input
                            type={showPassword ? 'text' : 'password'}
                            placeholder="Enter your password"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            disabled={isSubmitting}
                            bg={inputBg}
                            borderWidth="2px"
                            borderColor={borderColor}
                            borderRadius="xl"
                            py={6}
                            fontSize="md"
                            _hover={{
                              borderColor: inputHoverBorderColor
                            }}
                            _focus={{
                              borderColor: accentColor,
                              boxShadow: `0 0 0 1px ${accentColor}`,
                              bg: inputBg
                            }}
                            _placeholder={{
                              color: inputPlaceholderColor
                            }}
                          />
                          <InputRightElement top={2} right={2}>
                            <IconButton
                              variant="ghost"
                              size="sm"
                              aria-label={showPassword ? 'Hide password' : 'Show password'}
                              icon={showPassword ? <FiEyeOff /> : <FiEye />}
                              onClick={() => setShowPassword(!showPassword)}
                              borderRadius="lg"
                              _hover={{
                                bg: passwordToggleHoverBg
                              }}
                            />
                          </InputRightElement>
                        </InputGroup>
                      </FormControl>
                      
                      <Button
                        type="submit"
                        bg={accentColor}
                        color="white"
                        isLoading={isSubmitting}
                        loadingText="Signing in..."
                        width="full"
                        size="lg"
                        borderRadius="xl"
                        fontWeight="bold"
                        fontSize="md"
                        py={6}
                        leftIcon={<FiLogIn />}
                        _hover={{
                          bg: buttonHoverBg,
                          transform: 'translateY(-1px)',
                          boxShadow: 'lg'
                        }}
                        _active={{
                          transform: 'translateY(0)'
                        }}
                        transition="all 0.2s ease"
                        boxShadow="md"
                      >
                        Sign In
                      </Button>
                    </VStack>
                  </form>
                  
                  {/* Divider */}
                  <HStack my={8}>
                    <Divider borderColor={borderColor} />
                    <Text 
                      px={3} 
                      color={textColor} 
                      fontSize="sm" 
                      fontWeight="medium"
                      whiteSpace="nowrap"
                    >
                      Need help?
                    </Text>
                    <Divider borderColor={borderColor} />
                  </HStack>
                  
                  {/* Support Text */}
                  <Box
                    bg={supportPanelBg}
                    borderWidth="1px"
                    borderColor={supportPanelBorder}
                    borderRadius="xl"
                    p={4}
                  >
                    <VStack spacing={4} align="stretch">
                      <Text
                        textAlign="center"
                        color={headingColor}
                        fontSize="sm"
                        fontWeight="semibold"
                      >
                        Account access is managed by your administrator
                      </Text>

                      <SimpleGrid columns={{ base: 1, sm: 2 }} spacing={3}>
                        <HStack
                          align="start"
                          spacing={3}
                          bg={supportIconBg}
                          borderRadius="lg"
                          px={3}
                          py={3}
                        >
                          <Center
                            w={8}
                            h={8}
                            borderRadius="lg"
                            bg={useColorModeValue('blue.100', 'blue.800')}
                            flexShrink={0}
                          >
                            <FiUsers color={iconColor} size={16} />
                          </Center>
                          <VStack align="start" spacing={0}>
                            <Text color={headingColor} fontSize="sm" fontWeight="medium">
                              Need an account?
                            </Text>
                            <Text color={textColor} fontSize="xs" lineHeight="short">
                              Contact your admin or HR team to request user access.
                            </Text>
                          </VStack>
                        </HStack>

                        <HStack
                          align="start"
                          spacing={3}
                          bg={supportIconBg}
                          borderRadius="lg"
                          px={3}
                          py={3}
                        >
                          <Center
                            w={8}
                            h={8}
                            borderRadius="lg"
                            bg={useColorModeValue('green.100', 'green.800')}
                            flexShrink={0}
                          >
                            <FiShield color={iconColor} size={16} />
                          </Center>
                          <VStack align="start" spacing={0}>
                            <Text color={headingColor} fontSize="sm" fontWeight="medium">
                              Forgot your password?
                            </Text>
                            <Text color={textColor} fontSize="xs" lineHeight="short">
                              Ask your system administrator to reset your credentials.
                            </Text>
                          </VStack>
                        </HStack>
                      </SimpleGrid>

                      <HStack justify="center" spacing={2}>
                        <FiMail color={iconColor} size={14} />
                        <Text
                          textAlign="center"
                          color={textColor}
                          fontSize="xs"
                          lineHeight="relaxed"
                        >
                          For login assistance, please contact your internal IT or system administrator.
                        </Text>
                      </HStack>
                    </VStack>
                  </Box>
                </CardBody>
              </Card>
            </Box>
          </Flex>
        </SimpleGrid>
      </Container>
    </Box>
  );
};

export default function LoginPage() {
  return (
    <SimpleLayout showSidebar={false} requireAuth={false}>
      <LoginContent />
    </SimpleLayout>
  );
}
