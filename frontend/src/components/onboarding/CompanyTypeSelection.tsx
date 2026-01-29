import React, { useState } from 'react';
import {
    Box,
    Heading,
    Text,
    SimpleGrid,
    Button,
    VStack,
    Icon,
    useColorModeValue,
    Badge,
    useToast,
    Container,
    Flex,
} from '@chakra-ui/react';
import { FiBriefcase, FiShoppingBag, FiTool, FiCheck } from 'react-icons/fi';
import { useTranslation } from '@/hooks/useTranslation';
import axios from 'axios';
import { API_BASE_URL, API_ENDPOINTS } from '@/config/api';
import { useAuth } from '@/contexts/AuthContext';

interface CompanyTypeSelectionProps {
    onSuccess: () => void;
}

const CompanyTypeSelection: React.FC<CompanyTypeSelectionProps> = ({ onSuccess }) => {
    const { t } = useTranslation();
    const { token } = useAuth();
    const toast = useToast();
    const [selectedType, setSelectedType] = useState<string | null>(null);
    const [isSubmitting, setIsSubmitting] = useState(false);

    const cardBg = useColorModeValue('white', 'gray.800');
    const borderColor = useColorModeValue('gray.200', 'gray.700');
    const selectedBorderColor = 'blue.500';
    const hoverBorderColor = useColorModeValue('blue.300', 'blue.400');

    const companyTypes = [
        {
            id: 'SERVICE',
            title: 'Perusahaan Jasa',
            icon: FiBriefcase,
            description: 'Cocok untuk konsultan, freelancer, agensi, dan bisnis berbasis layanan.',
            features: ['Pendapatan Jasa', 'Beban Operasional', 'Tanpa Stok Barang'],
        },
        {
            id: 'TRADING',
            title: 'Perusahaan Dagang',
            icon: FiShoppingBag,
            description: 'Cocok untuk toko retail, grosir, distributor, dan e-commerce.',
            features: ['Stok Barang Dagang', 'Harga Pokok Penjualan', 'Laba Kotor'],
        },
        {
            id: 'MANUFACTURING',
            title: 'Perusahaan Manufaktur',
            icon: FiTool,
            description: 'Cocok untuk pabrik, produksi, dan pengolahan bahan baku.',
            features: ['Bahan Baku & WIP', 'Overhead Pabrik', 'Harga Pokok Produksi'],
        },
    ];

    const handleSelect = (type: string) => {
        setSelectedType(type);
    };

    const handleSubmit = async () => {
        if (!selectedType || !token) return;

        setIsSubmitting(true);
        try {
            await axios.post(
                `${API_BASE_URL}${API_ENDPOINTS.SETTINGS}/initialize-coa`,
                { company_type: selectedType },
                { headers: { Authorization: `Bearer ${token}` } }
            );

            toast({
                title: 'Sukses',
                description: 'Jenis perusahaan berhasil disimpan. COA standar telah dibuat.',
                status: 'success',
                duration: 5000,
                isClosable: true,
            });

            onSuccess();
        } catch (error: any) {
            toast({
                title: 'Gagal',
                description: error.response?.data?.error || 'Gagal menyimpan pengaturan.',
                status: 'error',
                duration: 5000,
                isClosable: true,
            });
        } finally {
            setIsSubmitting(false);
        }
    };

    return (
        <Container maxW="container.lg" py={10}>
            <VStack spacing={8} align="center">
                <Box textAlign="center">
                    <Heading size="xl" mb={4}>Pilih Jenis Perusahaan Anda</Heading>
                    <Text fontSize="lg" color="gray.500">
                        Kami akan menyesuaikan Standar Akun (Chart of Accounts) berdasarkan pilihan ini.
                        <br />
                        <strong>Penting:</strong> Pilihan ini tidak dapat diubah setelah disimpan.
                    </Text>
                </Box>

                <SimpleGrid columns={{ base: 1, md: 3 }} spacing={6} w="full">
                    {companyTypes.map((type) => (
                        <Box
                            key={type.id}
                            bg={cardBg}
                            p={6}
                            borderRadius="xl"
                            borderWidth="2px"
                            borderColor={selectedType === type.id ? selectedBorderColor : borderColor}
                            cursor="pointer"
                            transition="all 0.2s"
                            _hover={{ borderColor: hoverBorderColor, transform: 'translateY(-4px)', shadow: 'md' }}
                            onClick={() => handleSelect(type.id)}
                            position="relative"
                        >
                            {selectedType === type.id && (
                                <Box position="absolute" top={4} right={4} color="blue.500">
                                    <Icon as={FiCheck} w={6} h={6} />
                                </Box>
                            )}
                            <VStack spacing={4} align="start">
                                <Icon as={type.icon} w={10} h={10} color="blue.500" />
                                <Heading size="md">{type.title}</Heading>
                                <Text color="gray.500" fontSize="sm">{type.description}</Text>
                                <VStack align="start" spacing={2} pt={2}>
                                    {type.features.map((feature, idx) => (
                                        <Badge key={idx} colorScheme="blue" variant="subtle" px={2} py={1} borderRadius="md">
                                            {feature}
                                        </Badge>
                                    ))}
                                </VStack>
                            </VStack>
                        </Box>
                    ))}
                </SimpleGrid>

                <Button
                    size="lg"
                    colorScheme="blue"
                    onClick={handleSubmit}
                    isDisabled={!selectedType}
                    isLoading={isSubmitting}
                    px={10}
                    fontSize="md"
                >
                    Simpan & Lanjutkan
                </Button>
            </VStack>
        </Container>
    );
};

export default CompanyTypeSelection;
