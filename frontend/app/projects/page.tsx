'use client';

import React, { useState, useEffect } from 'react';
import {
    Box,
    Button,
    Flex,
    Heading,
    Icon,
    Input,
    InputGroup,
    InputLeftElement,
    Table,
    Thead,
    Tbody,
    Tr,
    Th,
    Td,
    Text,
    useColorModeValue,
    useDisclosure,
    Modal,
    ModalOverlay,
    ModalContent,
    ModalHeader,
    ModalCloseButton,
    ModalBody,
    ModalFooter,
    FormControl,
    FormLabel,
    useToast,
    IconButton,
    Tooltip,
    Badge,
    Textarea,
    Select,
} from '@chakra-ui/react';
import { FiPlus, FiSearch, FiEdit2, FiTrash2, FiFileText } from 'react-icons/fi';
import Layout from '@/components/layout/Layout';
import ProtectedModule from '@/components/common/ProtectedModule';
import { projectService, Project, ProjectCreateRequest, ProjectUpdateRequest } from '@/services/projectService';
import { usePermissions } from '@/hooks/usePermissions';
import { useTranslation } from '@/hooks/useTranslation';
import CurrencyInput from '@/components/common/CurrencyInput';
import ProjectBOMList from '@/components/projects/ProjectBOMList';
import { Tabs, TabList, TabPanels, Tab, TabPanel } from '@chakra-ui/react';

export default function ProjectsPage() {
    const [projects, setProjects] = useState<Project[]>([]);
    const [loading, setLoading] = useState(true);
    const [searchQuery, setSearchQuery] = useState('');
    const [isSubmitting, setIsSubmitting] = useState(false);
    const { canCreate, canEdit, canDelete } = usePermissions();
    const { t } = useTranslation();
    const toast = useToast();

    const formatCurrency = (value: number) => {
        return new Intl.NumberFormat('id-ID', {
            style: 'currency',
            currency: 'IDR',
            minimumFractionDigits: 0,
            maximumFractionDigits: 0,
        }).format(value);
    };

    const { isOpen: isCreateOpen, onOpen: onCreateOpen, onClose: onCreateClose } = useDisclosure();
    const { isOpen: isEditOpen, onOpen: onEditOpen, onClose: onEditClose } = useDisclosure();
    const { isOpen: isDeleteOpen, onOpen: onDeleteOpen, onClose: onDeleteClose } = useDisclosure();

    const [selectedProject, setSelectedProject] = useState<Project | null>(null);

    const [formData, setFormData] = useState({
        code: '',
        name: '',
        description: '',
        status: 'ACTIVE',
        start_date: '',
        end_date: '',
        total_budget: '0',
    });

    // Theme colors
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

    const fetchProjects = async () => {
        try {
            setLoading(true);
            const data = await projectService.getAll();
            setProjects(data || []);
        } catch (error) {
            toast({
                title: 'Gagal memuat proyek',
                status: 'error',
                duration: 3000,
                isClosable: true,
            });
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchProjects();
    }, []);

    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>) => {
        const { name, value } = e.target;
        setFormData((prev) => ({ ...prev, [name]: value }));
    };

    const handleOpenCreate = () => {
        setFormData({
            code: '',
            name: '',
            description: '',
            status: 'ACTIVE',
            start_date: '',
            end_date: '',
            total_budget: '0',
        });
        onCreateOpen();
    };

    const handleOpenEdit = (project: Project) => {
        setSelectedProject(project);
        setFormData({
            code: project.code,
            name: project.name,
            description: project.description || '',
            status: project.status,
            start_date: project.start_date ? project.start_date.split('T')[0] : '',
            end_date: project.end_date ? project.end_date.split('T')[0] : '',
            total_budget: project.total_budget.toString(),
        });
        onEditOpen();
    };

    const handleOpenDelete = (project: Project) => {
        setSelectedProject(project);
        onDeleteOpen();
    };

    const handleCreate = async () => {
        try {
            setIsSubmitting(true);
            const payload: ProjectCreateRequest = {
                code: formData.code,
                name: formData.name,
                description: formData.description,
                start_date: formData.start_date ? new Date(formData.start_date).toISOString() : undefined,
                end_date: formData.end_date ? new Date(formData.end_date).toISOString() : undefined,
                total_budget: parseFloat(formData.total_budget) || 0,
            };
            await projectService.create(payload);
            toast({ title: 'Proyek berhasil dibuat', status: 'success', duration: 3000 });
            onCreateClose();
            fetchProjects();
        } catch (error) {
            toast({ title: 'Gagal membuat proyek', status: 'error', duration: 3000 });
        } finally {
            setIsSubmitting(false);
        }
    };

    const handleEdit = async () => {
        if (!selectedProject) return;
        try {
            setIsSubmitting(true);
            const payload: ProjectUpdateRequest = {
                name: formData.name,
                description: formData.description,
                status: formData.status,
                start_date: formData.start_date ? new Date(formData.start_date).toISOString() : undefined,
                end_date: formData.end_date ? new Date(formData.end_date).toISOString() : undefined,
                total_budget: parseFloat(formData.total_budget) || 0,
            };
            await projectService.update(selectedProject.id, payload);
            toast({ title: 'Proyek berhasil diperbarui', status: 'success', duration: 3000 });
            onEditClose();
            fetchProjects();
        } catch (error) {
            toast({ title: 'Gagal memperbarui proyek', status: 'error', duration: 3000 });
        } finally {
            setIsSubmitting(false);
        }
    };

    const handleDelete = async () => {
        if (!selectedProject) return;
        try {
            setIsSubmitting(true);
            await projectService.delete(selectedProject.id);
            toast({ title: 'Proyek berhasil dihapus', status: 'success', duration: 3000 });
            onDeleteClose();
            fetchProjects();
        } catch (error) {
            toast({ title: 'Gagal menghapus proyek', status: 'error', duration: 3000 });
        } finally {
            setIsSubmitting(false);
        }
    };

    const filteredProjects = projects.filter(
        (p) =>
            p.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
            p.code.toLowerCase().includes(searchQuery.toLowerCase())
    );

    const getStatusBadge = (status: string) => {
        const statusMap: Record<string, { color: string; label: string }> = {
            ACTIVE: { color: 'green', label: 'Aktif' },
            COMPLETED: { color: 'blue', label: 'Selesai' },
            ON_HOLD: { color: 'orange', label: 'Ditunda' },
            CANCELLED: { color: 'red', label: 'Dibatalkan' },
        };
        const s = statusMap[status] || { color: 'gray', label: status };
        return <Badge colorScheme={s.color}>{s.label}</Badge>;
    };

    return (
        <ProtectedModule module="projects">
            <Layout>
                <Box p={6}>
                    <Flex justify="space-between" align="center" mb={6}>
                        <Box>
                            <Heading size="lg" color={textColor}>Manajemen Proyek</Heading>
                            <Text color={mutedTextColor} mt={1}>Kelola daftar proyek, biaya aktual, pendapatan diakui, dan pipeline penjualannya</Text>
                        </Box>
                        {canCreate('projects') && (
                            <Button
                                leftIcon={<FiPlus />}
                                colorScheme="blue"
                                onClick={handleOpenCreate}
                            >
                                Proyek Baru
                            </Button>
                        )}
                    </Flex>

                    <Box bg={bgCard} rounded="lg" shadow="sm" borderWidth="1px" borderColor={borderColor}>
                        <Flex p={4} borderBottomWidth="1px" borderColor={borderColor}>
                            <InputGroup maxW="300px">
                                <InputLeftElement pointerEvents="none">
                                    <Icon as={FiSearch} color={mutedTextColor} />
                                </InputLeftElement>
                                <Input
                                    placeholder="Cari nama atau kode proyek..."
                                    value={searchQuery}
                                    onChange={(e) => setSearchQuery(e.target.value)}
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
                                            <Td colSpan={9} textAlign="center" py={4}>Memuat data...</Td>
                                        </Tr>
                                    ) : filteredProjects.length === 0 ? (
                                        <Tr>
                                            <Td colSpan={9} textAlign="center" py={4}>Tidak ada data proyek</Td>
                                        </Tr>
                                    ) : (
                                        filteredProjects.map((project) => (
                                            <Tr key={project.id} _hover={{ bg: tableRowHoverBg }}>
                                                <Td fontWeight="bold">{project.code}</Td>
                                                <Td>
                                                    <Text fontWeight="medium">{project.name}</Text>
                                                    {project.description && (
                                                        <Text fontSize="xs" color={mutedTextColor} noOfLines={1}>{project.description}</Text>
                                                    )}
                                                </Td>
                                                <Td>{getStatusBadge(project.status)}</Td>
                                                <Td isNumeric>{formatCurrency(project.total_budget)}</Td>
                                                <Td isNumeric color={project.total_actual_cost > project.total_budget ? 'red.500' : 'inherit'}>
                                                    {formatCurrency(project.total_actual_cost)}
                                                </Td>
                                                <Td isNumeric color="green.500" fontWeight="semibold">
                                                    {formatCurrency(project.total_sales_revenue || 0)}
                                                </Td>
                                                <Td isNumeric color={(project.sales_pipeline_amount || 0) > 0 ? 'orange.500' : 'gray.500'} fontWeight="medium">
                                                    {formatCurrency(project.sales_pipeline_amount || 0)}
                                                </Td>
                                                <Td>{project.start_date ? new Date(project.start_date).toLocaleDateString('id-ID') : '-'}</Td>
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

                {/* Modal Create Project */}
                <Modal isOpen={isCreateOpen} onClose={onCreateClose} size="xl">
                    <ModalOverlay />
                    <ModalContent bg={modalBg} borderWidth="1px" borderColor={borderColor}>
                        <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={borderColor}>Buat Proyek Baru</ModalHeader>
                        <ModalCloseButton color={textColor} />
                        <ModalBody>
                            <Flex direction="column" gap={4}>
                                <FormControl isRequired>
                                    <FormLabel color={textColor}>Kode Proyek</FormLabel>
                                    <Input name="code" value={formData.code} onChange={handleInputChange} placeholder="PROJ-001" bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles} _placeholder={{ color: mutedTextColor }} />
                                </FormControl>
                                <FormControl isRequired>
                                    <FormLabel color={textColor}>Nama Proyek</FormLabel>
                                    <Input name="name" value={formData.name} onChange={handleInputChange} placeholder="Pembangunan Gudang A" bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles} _placeholder={{ color: mutedTextColor }} />
                                </FormControl>
                                <FormControl>
                                    <FormLabel color={textColor}>Deskripsi</FormLabel>
                                    <Textarea name="description" value={formData.description} onChange={handleInputChange} bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles} />
                                </FormControl>
                                <Flex gap={4}>
                                    <FormControl>
                                        <FormLabel color={textColor}>Tanggal Mulai</FormLabel>
                                        <Input type="date" name="start_date" value={formData.start_date} onChange={handleInputChange} bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles} />
                                    </FormControl>
                                    <FormControl>
                                        <FormLabel color={textColor}>Tanggal Selesai</FormLabel>
                                        <Input type="date" name="end_date" value={formData.end_date} onChange={handleInputChange} bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles} />
                                    </FormControl>
                                </Flex>
                                <FormControl>
                                    <FormLabel color={textColor}>Total Anggaran (Estimasi)</FormLabel>
                                    <Text fontSize="sm" color={mutedTextColor} mb={2}>Anggaran akan dihitung secara otomatis setelah proyek dibuat dan BOM ditambahkan.</Text>
                                    <CurrencyInput
                                        value={0}
                                        onChange={() => { }}
                                        isDisabled
                                    />
                                </FormControl>
                            </Flex>
                        </ModalBody>
                        <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={borderColor}>
                            <Button variant="ghost" mr={3} onClick={onCreateClose}>Batal</Button>
                            <Button colorScheme="blue" onClick={handleCreate} isLoading={isSubmitting}>Simpan</Button>
                        </ModalFooter>
                    </ModalContent>
                </Modal>

                {/* Modal Edit Project */}
                <Modal isOpen={isEditOpen} onClose={onEditClose} size="4xl">
                    <ModalOverlay />
                    <ModalContent bg={modalBg} borderWidth="1px" borderColor={borderColor}>
                        <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={borderColor}>Detail & Edit Proyek: {selectedProject?.name}</ModalHeader>
                        <ModalCloseButton color={textColor} />

                        <Tabs colorScheme="blue" mx={6} mt={2}>
                            <TabList borderColor={borderColor}>
                                <Tab fontWeight="semibold" _selected={{ color: tabSelectedColor, bg: tabSelectedBg, borderColor: borderColor }}>Informasi Umum</Tab>
                                <Tab fontWeight="semibold" _selected={{ color: tabSelectedColor, bg: tabSelectedBg, borderColor: borderColor }}>Bill of Materials (BOM)</Tab>
                            </TabList>

                            <TabPanels>
                                <TabPanel px={0}>
                                    <ModalBody>
                                        <Flex direction="column" gap={4}>
                                            <FormControl isRequired>
                                                <FormLabel color={textColor}>Kode Proyek</FormLabel>
                                                <Input name="code" value={formData.code} isReadOnly bg={readOnlyBg} borderColor={inputBorderColor} color={textColor} />
                                            </FormControl>
                                            <FormControl isRequired>
                                                <FormLabel color={textColor}>Nama Proyek</FormLabel>
                                                <Input name="name" value={formData.name} onChange={handleInputChange} bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles} />
                                            </FormControl>
                                            <FormControl>
                                                <FormLabel color={textColor}>Status</FormLabel>
                                                <Select name="status" value={formData.status} onChange={handleInputChange} bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles}>
                                                    <option value="ACTIVE">Aktif</option>
                                                    <option value="COMPLETED">Selesai</option>
                                                    <option value="ON_HOLD">Ditunda</option>
                                                    <option value="CANCELLED">Dibatalkan</option>
                                                </Select>
                                            </FormControl>
                                            <FormControl>
                                                <FormLabel color={textColor}>Deskripsi</FormLabel>
                                                <Textarea name="description" value={formData.description} onChange={handleInputChange} bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles} />
                                            </FormControl>
                                            <Flex gap={4}>
                                                <FormControl>
                                                    <FormLabel color={textColor}>Tanggal Mulai</FormLabel>
                                                    <Input type="date" name="start_date" value={formData.start_date} onChange={handleInputChange} bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles} />
                                                </FormControl>
                                                <FormControl>
                                                    <FormLabel color={textColor}>Tanggal Selesai</FormLabel>
                                                    <Input type="date" name="end_date" value={formData.end_date} onChange={handleInputChange} bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles} />
                                                </FormControl>
                                            </Flex>
                                            <FormControl>
                                                <FormLabel color={textColor}>Total Anggaran As-Built</FormLabel>
                                                <Text fontSize="sm" color={mutedTextColor} mb={1}>Dihitung otomatis dari total BOM yang dilampirkan.</Text>
                                                <CurrencyInput
                                                    value={parseFloat(formData.total_budget) || 0}
                                                    onChange={() => { }}
                                                    isDisabled
                                                />
                                            </FormControl>
                                        </Flex>
                                    </ModalBody>
                                    <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={borderColor}>
                                        <Button variant="ghost" mr={3} onClick={onEditClose}>Batal</Button>
                                        <Button colorScheme="blue" onClick={handleEdit} isLoading={isSubmitting}>Simpan Perubahan</Button>
                                    </ModalFooter>
                                </TabPanel>

                                <TabPanel px={0}>
                                    <ModalBody pb={6}>
                                        {selectedProject && (
                                            <ProjectBOMList
                                                projectId={selectedProject.id}
                                                canEdit={canEdit('projects')}
                                                onBudgetUpdated={fetchProjects}
                                            />
                                        )}
                                    </ModalBody>
                                </TabPanel>
                            </TabPanels>
                        </Tabs>
                    </ModalContent>
                </Modal>

                {/* Modal Delete */}
                <Modal isOpen={isDeleteOpen} onClose={onDeleteClose}>
                    <ModalOverlay />
                    <ModalContent bg={modalBg} borderWidth="1px" borderColor={borderColor}>
                        <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={borderColor}>Hapus Proyek</ModalHeader>
                        <ModalCloseButton color={textColor} />
                        <ModalBody>
                            <Text color={textColor}>Apakah Anda yakin ingin menghapus proyek <strong>{selectedProject?.name}</strong>? Tindakan ini tidak dapat dibatalkan.</Text>
                        </ModalBody>
                        <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={borderColor}>
                            <Button variant="ghost" mr={3} onClick={onDeleteClose}>Batal</Button>
                            <Button colorScheme="red" onClick={handleDelete} isLoading={isSubmitting}>Ya, Hapus</Button>
                        </ModalFooter>
                    </ModalContent>
                </Modal>

            </Layout>
        </ProtectedModule>
    );
}
