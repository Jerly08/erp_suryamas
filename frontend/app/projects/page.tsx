'use client';

import React, { useEffect, useState } from 'react';
import {
    Badge,
    Box,
    Button,
    Flex,
    FormControl,
    FormLabel,
    Heading,
    Icon,
    IconButton,
    Input,
    InputGroup,
    InputLeftElement,
    Modal,
    ModalBody,
    ModalCloseButton,
    ModalContent,
    ModalFooter,
    ModalHeader,
    ModalOverlay,
    Select,
    Table,
    Tab,
    TabList,
    TabPanel,
    TabPanels,
    Tabs,
    Tbody,
    Td,
    Text,
    Textarea,
    Th,
    Thead,
    Tooltip,
    Tr,
    useColorModeValue,
    useDisclosure,
    useToast,
} from '@chakra-ui/react';
import { FiEdit2, FiPlus, FiSearch, FiTrash2 } from 'react-icons/fi';
import ProtectedModule from '@/components/common/ProtectedModule';
import CurrencyInput from '@/components/common/CurrencyInput';
import Layout from '@/components/layout/Layout';
import ProjectBOMList from '@/components/projects/ProjectBOMList';
import { usePermissions } from '@/hooks/usePermissions';
import {
    Project,
    ProjectCreateRequest,
    ProjectUpdateRequest,
    projectService,
} from '@/services/projectService';

type ProjectFormState = {
    code: string;
    name: string;
    description: string;
    status: Project['status'];
    start_date: string;
    end_date: string;
    total_budget: string;
};

const defaultProjectForm: ProjectFormState = {
    code: '',
    name: '',
    description: '',
    status: 'ACTIVE',
    start_date: '',
    end_date: '',
    total_budget: '0',
};

const toDateInputValue = (value: string | null | undefined): string => {
    if (!value) {
        return '';
    }
    return value.split('T')[0];
};

const serializeDateInput = (value: string): string | undefined => {
    if (!value) {
        return undefined;
    }
    return `${value}T00:00:00Z`;
};

const isInvalidDateRange = (startDate: string, endDate: string): boolean => {
    return Boolean(startDate && endDate && endDate < startDate);
};

const extractErrorMessage = (error: any, fallback: string): string => {
    return (
        error?.response?.data?.error ||
        error?.response?.data?.message ||
        error?.message ||
        fallback
    );
};

const buildProjectForm = (project: Project): ProjectFormState => ({
    code: project.code,
    name: project.name,
    description: project.description || '',
    status: project.status,
    start_date: toDateInputValue(project.start_date),
    end_date: toDateInputValue(project.end_date),
    total_budget: String(project.total_budget || 0),
});

export default function ProjectsPage() {
    const [projects, setProjects] = useState<Project[]>([]);
    const [loading, setLoading] = useState(true);
    const [searchQuery, setSearchQuery] = useState('');
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [selectedProject, setSelectedProject] = useState<Project | null>(null);
    const [formData, setFormData] = useState<ProjectFormState>(defaultProjectForm);

    const { canCreate, canDelete, canEdit } = usePermissions();
    const toast = useToast();

    const { isOpen: isCreateOpen, onOpen: onCreateOpen, onClose: onCreateClose } = useDisclosure();
    const { isOpen: isEditOpen, onOpen: onEditOpen, onClose: onEditOpenClose } = useDisclosure();
    const { isOpen: isDeleteOpen, onOpen: onDeleteOpen, onClose: onDeleteClose } = useDisclosure();

    const bgCard = useColorModeValue('white', 'gray.800');
    const textColor = useColorModeValue('gray.700', 'white');
    const mutedTextColor = useColorModeValue('gray.500', 'gray.400');
    const borderColor = useColorModeValue('gray.200', 'gray.700');
    const tableHeaderBg = useColorModeValue('gray.50', 'gray.700');
    const inputBg = useColorModeValue('white', 'gray.700');
    const inputBorderColor = useColorModeValue('gray.300', 'gray.600');
    const tableRowHoverBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const modalBg = useColorModeValue('white', 'gray.800');
    const modalHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const modalFooterBg = useColorModeValue('gray.50', 'whiteAlpha.50');
    const readOnlyBg = useColorModeValue('gray.100', 'whiteAlpha.100');
    const tabSelectedBg = useColorModeValue('blue.50', 'blue.900');
    const tabSelectedColor = useColorModeValue('blue.600', 'blue.200');
    const fieldFocusStyles = {
        borderColor: 'blue.400',
        boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)',
    };

    const formatCurrency = (value: number) => {
        return new Intl.NumberFormat('id-ID', {
            style: 'currency',
            currency: 'IDR',
            minimumFractionDigits: 0,
            maximumFractionDigits: 0,
        }).format(value);
    };

    const fetchProjects = async (showErrorToast = true): Promise<Project[]> => {
        try {
            setLoading(true);
            const data = await projectService.getAll();
            const nextProjects = data || [];
            setProjects(nextProjects);
            return nextProjects;
        } catch (error) {
            if (showErrorToast) {
                toast({
                    title: 'Gagal memuat proyek',
                    description: extractErrorMessage(error, 'Data proyek tidak dapat dimuat'),
                    status: 'error',
                    duration: 4000,
                    isClosable: true,
                });
            }
            return [];
        } finally {
            setLoading(false);
        }
    };

    const refreshSelectedProject = async (projectId: number, showErrorToast = true): Promise<Project | null> => {
        try {
            const project = await projectService.getById(projectId);
            setSelectedProject(project);
            setFormData(buildProjectForm(project));
            return project;
        } catch (error) {
            if (showErrorToast) {
                toast({
                    title: 'Gagal menyegarkan detail proyek',
                    description: extractErrorMessage(error, 'Detail proyek tidak dapat dimuat'),
                    status: 'error',
                    duration: 4000,
                    isClosable: true,
                });
            }
            return null;
        }
    };

    const refreshProjectContext = async (projectId: number): Promise<void> => {
        await Promise.all([
            fetchProjects(false),
            refreshSelectedProject(projectId, false),
        ]);
    };

    useEffect(() => {
        fetchProjects();
    }, []);

    const handleInputChange = (
        event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>
    ) => {
        const { name, value } = event.target;
        setFormData((prev) => ({ ...prev, [name]: value }));
    };

    const handleBudgetChange = (value: number) => {
        setFormData((prev) => ({ ...prev, total_budget: String(value || 0) }));
    };

    const handleOpenCreate = () => {
        setFormData(defaultProjectForm);
        onCreateOpen();
    };

    const handleCloseEdit = () => {
        setSelectedProject(null);
        setFormData(defaultProjectForm);
        onEditOpenClose();
    };

    const handleOpenEdit = (project: Project) => {
        setSelectedProject(project);
        setFormData(buildProjectForm(project));
        onEditOpen();
    };

    const handleOpenDelete = (project: Project) => {
        setSelectedProject(project);
        onDeleteOpen();
    };

    const handleCreate = async () => {
        if (isInvalidDateRange(formData.start_date, formData.end_date)) {
            toast({
                title: 'Rentang tanggal tidak valid',
                description: 'Tanggal selesai tidak boleh lebih awal dari tanggal mulai.',
                status: 'warning',
                duration: 4000,
                isClosable: true,
            });
            return;
        }

        try {
            setIsSubmitting(true);
            const payload: ProjectCreateRequest = {
                code: formData.code.trim(),
                name: formData.name.trim(),
                description: formData.description.trim(),
                start_date: serializeDateInput(formData.start_date),
                end_date: serializeDateInput(formData.end_date),
                total_budget: Number(formData.total_budget) || 0,
            };
            await projectService.create(payload);
            toast({
                title: 'Proyek berhasil dibuat',
                status: 'success',
                duration: 3000,
                isClosable: true,
            });
            onCreateClose();
            setFormData(defaultProjectForm);
            await fetchProjects(false);
        } catch (error) {
            toast({
                title: 'Gagal membuat proyek',
                description: extractErrorMessage(error, 'Proyek tidak dapat dibuat'),
                status: 'error',
                duration: 4000,
                isClosable: true,
            });
        } finally {
            setIsSubmitting(false);
        }
    };

    const handleEdit = async () => {
        if (!selectedProject) {
            return;
        }

        if (isInvalidDateRange(formData.start_date, formData.end_date)) {
            toast({
                title: 'Rentang tanggal tidak valid',
                description: 'Tanggal selesai tidak boleh lebih awal dari tanggal mulai.',
                status: 'warning',
                duration: 4000,
                isClosable: true,
            });
            return;
        }

        try {
            setIsSubmitting(true);
            const payload: ProjectUpdateRequest = {
                name: formData.name.trim(),
                description: formData.description.trim(),
                status: formData.status,
                start_date: serializeDateInput(formData.start_date),
                end_date: serializeDateInput(formData.end_date),
                clear_start_date: !formData.start_date,
                clear_end_date: !formData.end_date,
                total_budget: Number(formData.total_budget) || 0,
            };
            const updatedProject = await projectService.update(selectedProject.id, payload);

            toast({
                title: 'Proyek berhasil diperbarui',
                status: 'success',
                duration: 3000,
                isClosable: true,
            });

            await Promise.all([
                fetchProjects(false),
                refreshSelectedProject(updatedProject.id, false),
            ]);
            handleCloseEdit();
        } catch (error) {
            toast({
                title: 'Gagal memperbarui proyek',
                description: extractErrorMessage(error, 'Perubahan proyek tidak dapat disimpan'),
                status: 'error',
                duration: 4000,
                isClosable: true,
            });
        } finally {
            setIsSubmitting(false);
        }
    };

    const handleDelete = async () => {
        if (!selectedProject) {
            return;
        }

        try {
            setIsSubmitting(true);
            await projectService.delete(selectedProject.id);
            toast({
                title: 'Proyek berhasil dihapus',
                status: 'success',
                duration: 3000,
                isClosable: true,
            });
            onDeleteClose();
            setSelectedProject(null);
            await fetchProjects(false);
        } catch (error) {
            toast({
                title: 'Gagal menghapus proyek',
                description: extractErrorMessage(
                    error,
                    'Proyek tidak dapat dihapus karena masih memiliki relasi data'
                ),
                status: 'error',
                duration: 4000,
                isClosable: true,
            });
        } finally {
            setIsSubmitting(false);
        }
    };

    const filteredProjects = projects.filter((project) => {
        const query = searchQuery.toLowerCase();
        return (
            project.name.toLowerCase().includes(query) ||
            project.code.toLowerCase().includes(query)
        );
    });

    const getStatusBadge = (status: string) => {
        const statusMap: Record<string, { color: string; label: string }> = {
            ACTIVE: { color: 'green', label: 'Aktif' },
            COMPLETED: { color: 'blue', label: 'Selesai' },
            ON_HOLD: { color: 'orange', label: 'Ditunda' },
            CANCELLED: { color: 'red', label: 'Dibatalkan' },
        };
        const config = statusMap[status] || { color: 'gray', label: status };
        return <Badge colorScheme={config.color}>{config.label}</Badge>;
    };

    return (
        <ProtectedModule module="projects">
            <Layout>
                <Box p={6}>
                    <Flex justify="space-between" align="center" mb={6}>
                        <Box>
                            <Heading size="lg" color={textColor}>Manajemen Proyek</Heading>
                            <Text color={mutedTextColor} mt={1}>
                                Kelola proyek, BOM, biaya aktual dari purchase, pendapatan diakui, dan pipeline penjualan
                            </Text>
                        </Box>
                        {canCreate('projects') && (
                            <Button leftIcon={<FiPlus />} colorScheme="blue" onClick={handleOpenCreate}>
                                Proyek Baru
                            </Button>
                        )}
                    </Flex>

                    <Box bg={bgCard} rounded="lg" shadow="sm" borderWidth="1px" borderColor={borderColor}>
                        <Flex p={4} borderBottomWidth="1px" borderColor={borderColor}>
                            <InputGroup maxW="320px">
                                <InputLeftElement pointerEvents="none">
                                    <Icon as={FiSearch} color={mutedTextColor} />
                                </InputLeftElement>
                                <Input
                                    placeholder="Cari nama atau kode proyek..."
                                    value={searchQuery}
                                    onChange={(event) => setSearchQuery(event.target.value)}
                                    bg={inputBg}
                                    borderColor={inputBorderColor}
                                    color={textColor}
                                    _hover={{ borderColor: inputBorderColor }}
                                    _focusVisible={fieldFocusStyles}
                                    _placeholder={{ color: mutedTextColor }}
                                />
                            </InputGroup>
                        </Flex>

                        <Box overflowX="auto">
                            <Table variant="simple">
                                <Thead bg={tableHeaderBg}>
                                    <Tr>
                                        <Th>Kode</Th>
                                        <Th>Nama Proyek</Th>
                                        <Th>Status</Th>
                                        <Th isNumeric>Total Anggaran</Th>
                                        <Th isNumeric>Biaya Aktual</Th>
                                        <Th isNumeric>Pendapatan Diakui</Th>
                                        <Th isNumeric>Pipeline Penjualan</Th>
                                        <Th>Tanggal Mulai</Th>
                                        <Th>Aksi</Th>
                                    </Tr>
                                </Thead>
                                <Tbody>
                                    {loading ? (
                                        <Tr>
                                            <Td colSpan={9} textAlign="center" py={4}>
                                                Memuat data...
                                            </Td>
                                        </Tr>
                                    ) : filteredProjects.length === 0 ? (
                                        <Tr>
                                            <Td colSpan={9} textAlign="center" py={4}>
                                                Tidak ada data proyek
                                            </Td>
                                        </Tr>
                                    ) : (
                                        filteredProjects.map((project) => (
                                            <Tr key={project.id} _hover={{ bg: tableRowHoverBg }}>
                                                <Td fontWeight="bold">{project.code}</Td>
                                                <Td>
                                                    <Text fontWeight="medium">{project.name}</Text>
                                                    {project.description && (
                                                        <Text fontSize="xs" color={mutedTextColor} noOfLines={1}>
                                                            {project.description}
                                                        </Text>
                                                    )}
                                                </Td>
                                                <Td>{getStatusBadge(project.status)}</Td>
                                                <Td isNumeric>{formatCurrency(project.total_budget)}</Td>
                                                <Td
                                                    isNumeric
                                                    color={
                                                        project.total_budget > 0 &&
                                                        project.total_actual_cost > project.total_budget
                                                            ? 'red.500'
                                                            : 'inherit'
                                                    }
                                                >
                                                    {formatCurrency(project.total_actual_cost)}
                                                </Td>
                                                <Td isNumeric color="green.500" fontWeight="semibold">
                                                    {formatCurrency(project.total_sales_revenue || 0)}
                                                </Td>
                                                <Td
                                                    isNumeric
                                                    color={(project.sales_pipeline_amount || 0) > 0 ? 'orange.500' : 'gray.500'}
                                                    fontWeight="medium"
                                                >
                                                    {formatCurrency(project.sales_pipeline_amount || 0)}
                                                </Td>
                                                <Td>
                                                    {project.start_date
                                                        ? new Date(project.start_date).toLocaleDateString('id-ID')
                                                        : '-'}
                                                </Td>
                                                <Td>
                                                    <Flex gap={2}>
                                                        {canEdit('projects') && (
                                                            <Tooltip label="Edit">
                                                                <IconButton
                                                                    aria-label="Edit project"
                                                                    icon={<FiEdit2 />}
                                                                    size="sm"
                                                                    variant="ghost"
                                                                    colorScheme="blue"
                                                                    onClick={() => handleOpenEdit(project)}
                                                                />
                                                            </Tooltip>
                                                        )}
                                                        {canDelete('projects') && (
                                                            <Tooltip label="Hapus">
                                                                <IconButton
                                                                    aria-label="Delete project"
                                                                    icon={<FiTrash2 />}
                                                                    size="sm"
                                                                    variant="ghost"
                                                                    colorScheme="red"
                                                                    onClick={() => handleOpenDelete(project)}
                                                                />
                                                            </Tooltip>
                                                        )}
                                                    </Flex>
                                                </Td>
                                            </Tr>
                                        ))
                                    )}
                                </Tbody>
                            </Table>
                        </Box>
                    </Box>
                </Box>

                <Modal isOpen={isCreateOpen} onClose={onCreateClose} size="xl">
                    <ModalOverlay />
                    <ModalContent bg={modalBg} borderWidth="1px" borderColor={borderColor}>
                        <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={borderColor}>
                            Buat Proyek Baru
                        </ModalHeader>
                        <ModalCloseButton color={textColor} />
                        <ModalBody>
                            <Flex direction="column" gap={4}>
                                <FormControl isRequired>
                                    <FormLabel color={textColor}>Kode Proyek</FormLabel>
                                    <Input
                                        name="code"
                                        value={formData.code}
                                        onChange={handleInputChange}
                                        placeholder="PROJ-001"
                                        bg={inputBg}
                                        borderColor={inputBorderColor}
                                        color={textColor}
                                        _hover={{ borderColor: inputBorderColor }}
                                        _focusVisible={fieldFocusStyles}
                                        _placeholder={{ color: mutedTextColor }}
                                    />
                                </FormControl>
                                <FormControl isRequired>
                                    <FormLabel color={textColor}>Nama Proyek</FormLabel>
                                    <Input
                                        name="name"
                                        value={formData.name}
                                        onChange={handleInputChange}
                                        placeholder="Pembangunan Gudang A"
                                        bg={inputBg}
                                        borderColor={inputBorderColor}
                                        color={textColor}
                                        _hover={{ borderColor: inputBorderColor }}
                                        _focusVisible={fieldFocusStyles}
                                        _placeholder={{ color: mutedTextColor }}
                                    />
                                </FormControl>
                                <FormControl>
                                    <FormLabel color={textColor}>Deskripsi</FormLabel>
                                    <Textarea
                                        name="description"
                                        value={formData.description}
                                        onChange={handleInputChange}
                                        bg={inputBg}
                                        borderColor={inputBorderColor}
                                        color={textColor}
                                        _hover={{ borderColor: inputBorderColor }}
                                        _focusVisible={fieldFocusStyles}
                                    />
                                </FormControl>
                                <Flex gap={4}>
                                    <FormControl>
                                        <FormLabel color={textColor}>Tanggal Mulai</FormLabel>
                                        <Input
                                            type="date"
                                            name="start_date"
                                            value={formData.start_date}
                                            onChange={handleInputChange}
                                            bg={inputBg}
                                            borderColor={inputBorderColor}
                                            color={textColor}
                                            _hover={{ borderColor: inputBorderColor }}
                                            _focusVisible={fieldFocusStyles}
                                        />
                                    </FormControl>
                                    <FormControl isInvalid={isInvalidDateRange(formData.start_date, formData.end_date)}>
                                        <FormLabel color={textColor}>Tanggal Selesai</FormLabel>
                                        <Input
                                            type="date"
                                            name="end_date"
                                            value={formData.end_date}
                                            onChange={handleInputChange}
                                            bg={inputBg}
                                            borderColor={inputBorderColor}
                                            color={textColor}
                                            _hover={{ borderColor: inputBorderColor }}
                                            _focusVisible={fieldFocusStyles}
                                        />
                                    </FormControl>
                                </Flex>
                                <FormControl>
                                    <FormLabel color={textColor}>Total Anggaran</FormLabel>
                                    <Text fontSize="sm" color={mutedTextColor} mb={2}>
                                        Isi manual sesuai target anggaran proyek. BOM tetap dipakai untuk estimasi kebutuhan material.
                                    </Text>
                                    <CurrencyInput
                                        value={Number(formData.total_budget) || 0}
                                        onChange={handleBudgetChange}
                                    />
                                </FormControl>
                            </Flex>
                        </ModalBody>
                        <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={borderColor}>
                            <Button variant="ghost" mr={3} onClick={onCreateClose}>
                                Batal
                            </Button>
                            <Button colorScheme="blue" onClick={handleCreate} isLoading={isSubmitting}>
                                Simpan
                            </Button>
                        </ModalFooter>
                    </ModalContent>
                </Modal>

                <Modal isOpen={isEditOpen} onClose={handleCloseEdit} size="4xl">
                    <ModalOverlay />
                    <ModalContent bg={modalBg} borderWidth="1px" borderColor={borderColor}>
                        <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={borderColor}>
                            Detail & Edit Proyek: {selectedProject?.name}
                        </ModalHeader>
                        <ModalCloseButton color={textColor} />

                        <Tabs colorScheme="blue" mx={6} mt={2}>
                            <TabList borderColor={borderColor}>
                                <Tab
                                    fontWeight="semibold"
                                    _selected={{ color: tabSelectedColor, bg: tabSelectedBg, borderColor: borderColor }}
                                >
                                    Informasi Umum
                                </Tab>
                                <Tab
                                    fontWeight="semibold"
                                    _selected={{ color: tabSelectedColor, bg: tabSelectedBg, borderColor: borderColor }}
                                >
                                    Bill of Materials (BOM)
                                </Tab>
                            </TabList>

                            <TabPanels>
                                <TabPanel px={0}>
                                    <ModalBody>
                                        <Flex direction="column" gap={4}>
                                            <FormControl isRequired>
                                                <FormLabel color={textColor}>Kode Proyek</FormLabel>
                                                <Input
                                                    name="code"
                                                    value={formData.code}
                                                    isReadOnly
                                                    bg={readOnlyBg}
                                                    borderColor={inputBorderColor}
                                                    color={textColor}
                                                />
                                            </FormControl>
                                            <FormControl isRequired>
                                                <FormLabel color={textColor}>Nama Proyek</FormLabel>
                                                <Input
                                                    name="name"
                                                    value={formData.name}
                                                    onChange={handleInputChange}
                                                    bg={inputBg}
                                                    borderColor={inputBorderColor}
                                                    color={textColor}
                                                    _hover={{ borderColor: inputBorderColor }}
                                                    _focusVisible={fieldFocusStyles}
                                                />
                                            </FormControl>
                                            <FormControl>
                                                <FormLabel color={textColor}>Status</FormLabel>
                                                <Select
                                                    name="status"
                                                    value={formData.status}
                                                    onChange={handleInputChange}
                                                    bg={inputBg}
                                                    borderColor={inputBorderColor}
                                                    color={textColor}
                                                    _hover={{ borderColor: inputBorderColor }}
                                                    _focusVisible={fieldFocusStyles}
                                                >
                                                    <option value="ACTIVE">Aktif</option>
                                                    <option value="COMPLETED">Selesai</option>
                                                    <option value="ON_HOLD">Ditunda</option>
                                                    <option value="CANCELLED">Dibatalkan</option>
                                                </Select>
                                            </FormControl>
                                            <FormControl>
                                                <FormLabel color={textColor}>Deskripsi</FormLabel>
                                                <Textarea
                                                    name="description"
                                                    value={formData.description}
                                                    onChange={handleInputChange}
                                                    bg={inputBg}
                                                    borderColor={inputBorderColor}
                                                    color={textColor}
                                                    _hover={{ borderColor: inputBorderColor }}
                                                    _focusVisible={fieldFocusStyles}
                                                />
                                            </FormControl>
                                            <Flex gap={4}>
                                                <FormControl>
                                                    <FormLabel color={textColor}>Tanggal Mulai</FormLabel>
                                                    <Input
                                                        type="date"
                                                        name="start_date"
                                                        value={formData.start_date}
                                                        onChange={handleInputChange}
                                                        bg={inputBg}
                                                        borderColor={inputBorderColor}
                                                        color={textColor}
                                                        _hover={{ borderColor: inputBorderColor }}
                                                        _focusVisible={fieldFocusStyles}
                                                    />
                                                </FormControl>
                                                <FormControl isInvalid={isInvalidDateRange(formData.start_date, formData.end_date)}>
                                                    <FormLabel color={textColor}>Tanggal Selesai</FormLabel>
                                                    <Input
                                                        type="date"
                                                        name="end_date"
                                                        value={formData.end_date}
                                                        onChange={handleInputChange}
                                                        bg={inputBg}
                                                        borderColor={inputBorderColor}
                                                        color={textColor}
                                                        _hover={{ borderColor: inputBorderColor }}
                                                        _focusVisible={fieldFocusStyles}
                                                    />
                                                </FormControl>
                                            </Flex>
                                            <FormControl>
                                                <FormLabel color={textColor}>Total Anggaran</FormLabel>
                                                <Text fontSize="sm" color={mutedTextColor} mb={1}>
                                                    Dapat diubah manual. BOM membantu estimasi material, tetapi tidak menimpa anggaran proyek.
                                                </Text>
                                                <CurrencyInput
                                                    value={Number(formData.total_budget) || 0}
                                                    onChange={handleBudgetChange}
                                                />
                                            </FormControl>
                                            <FormControl>
                                                <FormLabel color={textColor}>Biaya Aktual</FormLabel>
                                                <Text fontSize="sm" color={mutedTextColor} mb={1}>
                                                    Hanya bertambah dari transaksi purchase yang terhubung ke proyek.
                                                </Text>
                                                <CurrencyInput
                                                    value={selectedProject?.total_actual_cost || 0}
                                                    onChange={() => {}}
                                                    isDisabled
                                                />
                                            </FormControl>
                                        </Flex>
                                    </ModalBody>
                                    <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={borderColor}>
                                        <Button variant="ghost" mr={3} onClick={handleCloseEdit}>
                                            Batal
                                        </Button>
                                        <Button colorScheme="blue" onClick={handleEdit} isLoading={isSubmitting}>
                                            Simpan Perubahan
                                        </Button>
                                    </ModalFooter>
                                </TabPanel>

                                <TabPanel px={0}>
                                    <ModalBody pb={6}>
                                        {selectedProject && (
                                            <ProjectBOMList
                                                projectId={selectedProject.id}
                                                canEdit={canEdit('projects')}
                                                canGeneratePR={canCreate('purchase_requests')}
                                                onBudgetUpdated={() => refreshProjectContext(selectedProject.id)}
                                            />
                                        )}
                                    </ModalBody>
                                </TabPanel>
                            </TabPanels>
                        </Tabs>
                    </ModalContent>
                </Modal>

                <Modal isOpen={isDeleteOpen} onClose={onDeleteClose}>
                    <ModalOverlay />
                    <ModalContent bg={modalBg} borderWidth="1px" borderColor={borderColor}>
                        <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={borderColor}>
                            Hapus Proyek
                        </ModalHeader>
                        <ModalCloseButton color={textColor} />
                        <ModalBody>
                            <Text color={textColor}>
                                Apakah Anda yakin ingin menghapus proyek <strong>{selectedProject?.name}</strong>?
                                Penghapusan akan ditolak jika proyek sudah punya relasi BOM, sales, purchase, atau purchase request.
                            </Text>
                        </ModalBody>
                        <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={borderColor}>
                            <Button variant="ghost" mr={3} onClick={onDeleteClose}>
                                Batal
                            </Button>
                            <Button colorScheme="red" onClick={handleDelete} isLoading={isSubmitting}>
                                Ya, Hapus
                            </Button>
                        </ModalFooter>
                    </ModalContent>
                </Modal>
            </Layout>
        </ProtectedModule>
    );
}
