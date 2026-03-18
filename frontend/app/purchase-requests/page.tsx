'use client';

import React, { useEffect, useRef, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
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
    Grid,
} from '@chakra-ui/react';
import { FiPlus, FiSearch, FiTrash2, FiCheck, FiX, FiEye } from 'react-icons/fi';
import Layout from '@/components/layout/Layout';
import ProtectedModule from '@/components/common/ProtectedModule';
import { usePermissions } from '@/hooks/usePermissions';
import { useTranslation } from '@/hooks/useTranslation';
import CurrencyInput from '@/components/common/CurrencyInput';
import { purchaseRequestService, PurchaseRequest, PurchaseRequestCreatePayload } from '@/services/purchaseRequestService';
import { projectService, Project } from '@/services/projectService';
import { useAuth } from '@/contexts/AuthContext';
import api from '@/services/api';

export default function PurchaseRequestsPage() {
    const router = useRouter();
    const searchParams = useSearchParams();
    const [requests, setRequests] = useState<PurchaseRequest[]>([]);
    const [projects, setProjects] = useState<Project[]>([]);
    const [products, setProducts] = useState<any[]>([]); // simplified type for catalogue items
    const [loading, setLoading] = useState(true);
    const [searchQuery, setSearchQuery] = useState('');
    const [isSubmitting, setIsSubmitting] = useState(false);
    const { canCreate, canEdit, canDelete, canApprove } = usePermissions();
    const { t } = useTranslation();
    const formatCurrency = (value: number) => {
        return new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(value);
    };
    const toast = useToast();
    const { user } = useAuth();
    const userDepartment = (user as { department?: string } | null)?.department || '';

    const { isOpen: isCreateOpen, onOpen: onCreateOpen, onClose: onCreateClose } = useDisclosure();
    const { isOpen: isViewOpen, onOpen: onViewOpen, onClose: onViewClose } = useDisclosure();
    const { isOpen: isRejectOpen, onOpen: onRejectOpen, onClose: onRejectClose } = useDisclosure();

    const [selectedRequest, setSelectedRequest] = useState<PurchaseRequest | null>(null);
    const [rejectReason, setRejectReason] = useState('');
    const handledNotificationRequestId = useRef<string | null>(null);

    // Form state
    const [department, setDepartment] = useState('');
    const [projectId, setProjectId] = useState('');
    const [notes, setNotes] = useState('');
    const [items, setItems] = useState([{ product_id: '', quantity: 1, estimated_price: 0, notes: '' }]);

    // Theme colors
    const bgCard = useColorModeValue('white', 'gray.800');
    const textColor = useColorModeValue('gray.700', 'white');
    const mutedTextColor = useColorModeValue('gray.500', 'gray.400');
    const borderColor = useColorModeValue('gray.200', 'gray.700');
    const tableHeaderBg = useColorModeValue('gray.50', 'gray.700');
    const inputBg = useColorModeValue('white', 'gray.700');
    const inputBorderColor = useColorModeValue('gray.300', 'gray.600');
    const modalBg = useColorModeValue('white', 'gray.800');
    const modalHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const modalFooterBg = useColorModeValue('gray.50', 'whiteAlpha.50');
    const infoPanelBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const notePanelBg = useColorModeValue('blue.50', 'blue.900');
    const notePanelColor = useColorModeValue('blue.800', 'blue.100');
    const rowHoverBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const fieldFocusStyles = {
        borderColor: 'blue.400',
        boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)',
    };

    useEffect(() => {
        fetchData();
    }, []);

    const fetchData = async () => {
        try {
            setLoading(true);
            const [prsRes, projectsRes, productsRes] = await Promise.all([
                purchaseRequestService.getAll(),
                projectService.getAll().catch(() => []),
                api.get('/api/v1/products').then(res => res.data.data).catch(() => [])
            ]);
            setRequests(prsRes || []);
            setProjects(projectsRes || []);
            setProducts(productsRes || []);
        } catch (error) {
            toast({ title: 'Gagal memuat data', status: 'error', duration: 3000 });
        } finally {
            setLoading(false);
        }
    };

    const handleOpenCreate = () => {
        setDepartment(userDepartment);
        setProjectId('');
        setNotes('');
        setItems([{ product_id: '', quantity: 1, estimated_price: 0, notes: '' }]);
        onCreateOpen();
    };

    const handleAddItem = () => {
        setItems([...items, { product_id: '', quantity: 1, estimated_price: 0, notes: '' }]);
    };

    const handleRemoveItem = (index: number) => {
        const newItems = items.filter((_, i) => i !== index);
        setItems(newItems.length ? newItems : [{ product_id: '', quantity: 1, estimated_price: 0, notes: '' }]);
    };

    const handleItemChange = (index: number, field: string, value: string | number) => {
        const newItems = [...items];
        if (field === 'product_id') {
            const selectedProd = products.find(p => p.id.toString() === value.toString());
            if (selectedProd) {
                newItems[index].estimated_price = selectedProd.purchase_price || 0;
            }
        }
        newItems[index] = { ...newItems[index], [field]: value } as any;
        setItems(newItems);
    };

    const handleCreateSubmit = async () => {
        // Basic validation
        if (!items.some(i => i.product_id)) {
            return toast({ title: 'Tambahkan minimal 1 item', status: 'warning', duration: 3000 });
        }

        try {
            setIsSubmitting(true);
            const payload: PurchaseRequestCreatePayload = {
                department,
                project_id: projectId ? parseInt(projectId) : undefined,
                notes,
                items: items.filter(i => i.product_id).map(i => ({
                    product_id: parseInt(i.product_id),
                    quantity: Number(i.quantity),
                    estimated_price: Number(i.estimated_price),
                    notes: i.notes
                }))
            };

            await purchaseRequestService.create(payload);
            toast({ title: 'Pengajuan berhasil dikirim', status: 'success', duration: 3000 });
            onCreateClose();
            fetchData();
        } catch (err: any) {
            toast({ title: 'Gagal membuat pengajuan', description: err.response?.data?.error || err.message, status: 'error', duration: 4000 });
        } finally {
            setIsSubmitting(false);
        }
    };

    const handleApprove = async (id: number) => {
        try {
            if (!window.confirm('Verifikasi Pagu Anggaran berhasil. Lanjut setujui?')) return;
            setIsSubmitting(true);
            await purchaseRequestService.approve(id);
            toast({ title: 'PR Disetujui', status: 'success', duration: 3000 });
            fetchData();
        } catch (err: any) {
            toast({ title: 'Gagal', description: err.response?.data?.error || err.message, status: 'error', duration: 3000 });
        } finally {
            setIsSubmitting(false);
        }
    };

    const handleRejectInit = (pr: PurchaseRequest) => {
        setSelectedRequest(pr);
        setRejectReason('');
        onRejectOpen();
    };

    const handleRejectSubmit = async () => {
        if (!selectedRequest || !rejectReason.trim()) return;
        try {
            setIsSubmitting(true);
            await purchaseRequestService.reject(selectedRequest.id, rejectReason);
            toast({ title: 'PR Ditolak', status: 'info', duration: 3000 });
            onRejectClose();
            fetchData();
        } catch (err: any) {
            toast({ title: 'Gagal tolak PR', description: err.response?.data?.error || err.message, status: 'error', duration: 3000 });
        } finally {
            setIsSubmitting(false);
        }
    };

    const handleView = async (pr: PurchaseRequest) => {
        try {
            setLoading(true);
            const detail = await purchaseRequestService.getById(pr.id);
            setSelectedRequest(detail);
            onViewOpen();
        } catch {
            toast({ title: 'Gagal mengambil detail PR', status: 'error', duration: 3000 });
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        const notificationRequestId = searchParams?.get('notificationRequestId');

        if (!notificationRequestId || handledNotificationRequestId.current === notificationRequestId) {
            return;
        }

        handledNotificationRequestId.current = notificationRequestId;

        const clearNotificationRequestParam = () => {
            const nextParams = new URLSearchParams(searchParams?.toString() || '');
            nextParams.delete('notificationRequestId');
            const nextUrl = nextParams.toString() ? `/purchase-requests?${nextParams.toString()}` : '/purchase-requests';
            router.replace(nextUrl);
        };

        const requestId = Number(notificationRequestId);
        if (!Number.isFinite(requestId)) {
            clearNotificationRequestParam();
            return;
        }

        const openRequestFromNotification = async () => {
            try {
                setLoading(true);
                const detail = await purchaseRequestService.getById(requestId);
                setSelectedRequest(detail);
                onViewOpen();
            } catch {
                toast({ title: 'Gagal membuka detail PR dari notifikasi', status: 'error', duration: 3000 });
            } finally {
                setLoading(false);
                clearNotificationRequestParam();
            }
        };

        void openRequestFromNotification();
    }, [onViewOpen, router, searchParams, toast]);

    const filteredRequests = requests.filter(r =>
        r.pr_number.toLowerCase().includes(searchQuery.toLowerCase()) ||
        (r.project?.name || '').toLowerCase().includes(searchQuery.toLowerCase())
    );

    const getStatusBadge = (status: string) => {
        switch (status) {
            case 'DRAFT': return <Badge colorScheme="gray">Draft</Badge>;
            case 'PENDING_APPROVAL': return <Badge colorScheme="orange">Pending</Badge>;
            case 'APPROVED': return <Badge colorScheme="green">Approved</Badge>;
            case 'REJECTED': return <Badge colorScheme="red">Rejected</Badge>;
            case 'COMPLETED': return <Badge colorScheme="blue">PO Processed</Badge>;
            default: return <Badge>{status}</Badge>;
        }
    };

    return (
        <ProtectedModule module="purchase_requests">
            <Layout>
                <Box p={6}>
                    <Flex justify="space-between" align="center" mb={6}>
                        <Box>
                            <Heading size="lg" color={textColor}>Purchase Requests</Heading>
                            <Text color={mutedTextColor} mt={1}>Kelola Pengajuan Pembelian Barang/Jasa</Text>
                        </Box>
                        {canCreate('purchase_requests') && (
                            <Button leftIcon={<FiPlus />} colorScheme="blue" onClick={handleOpenCreate}>
                                Buat Pengajuan
                            </Button>
                        )}
                    </Flex>

                    <Box bg={bgCard} rounded="lg" shadow="sm" borderWidth="1px" borderColor={borderColor}>
                        <Flex p={4} borderBottomWidth="1px" borderColor={borderColor}>
                            <InputGroup maxW="300px">
                                <InputLeftElement pointerEvents="none"><Icon as={FiSearch} color={mutedTextColor} /></InputLeftElement>
                                <Input placeholder="Cari by Nomor PR atau Proyek..." value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles} _placeholder={{ color: mutedTextColor }} />
                            </InputGroup>
                        </Flex>

                        <Box overflowX="auto">
                            <Table variant="simple">
                                <Thead bg={tableHeaderBg}>
                                    <Tr>
                                        <Th>PR Number</Th>
                                        <Th>Requester</Th>
                                        <Th>Departemen/Proyek</Th>
                                        <Th>Tgl Pengajuan</Th>
                                        <Th isNumeric>Estimasi Anggaran</Th>
                                        <Th>Status</Th>
                                        <Th textAlign="center">Aksi</Th>
                                    </Tr>
                                </Thead>
                                <Tbody>
                                    {loading ? (
                                        <Tr><Td colSpan={7} textAlign="center" py={4}>Memuat...</Td></Tr>
                                    ) : filteredRequests.length === 0 ? (
                                        <Tr><Td colSpan={7} textAlign="center" py={4}>Tidak ada pengajuan ditemukan</Td></Tr>
                                    ) : (
                                        filteredRequests.map((pr) => (
                                            <Tr key={pr.id} _hover={{ bg: rowHoverBg }}>
                                                <Td fontWeight="bold" color="blue.500" cursor="pointer" onClick={() => handleView(pr)}>
                                                    {pr.pr_number}
                                                </Td>
                                                <Td>{pr.requester?.first_name} {pr.requester?.last_name}</Td>
                                                <Td>
                                                    {pr.department && <Text fontSize="xs" color={mutedTextColor}>{pr.department}</Text>}
                                                    {pr.project && <Badge colorScheme="purple" fontSize="xs" mt={1}>{pr.project.code} - {pr.project.name}</Badge>}
                                                </Td>
                                                <Td>{new Date(pr.created_at).toLocaleDateString()}</Td>
                                                <Td isNumeric>{formatCurrency(pr.total_estimated_amount)}</Td>
                                                <Td>{getStatusBadge(pr.status)}</Td>
                                                <Td>
                                                    <Flex justify="center" gap={2}>
                                                        <Tooltip label="Lihat Detail">
                                                            <IconButton aria-label="View" icon={<FiEye />} size="sm" variant="ghost" onClick={() => handleView(pr)} />
                                                        </Tooltip>

                                                        {canApprove('purchase_requests') && pr.status === 'PENDING_APPROVAL' && (
                                                            <>
                                                                <Tooltip label="Setujui">
                                                                    <IconButton aria-label="Approve" icon={<FiCheck />} size="sm" colorScheme="green" variant="ghost" onClick={() => handleApprove(pr.id)} isLoading={isSubmitting} />
                                                                </Tooltip>
                                                                <Tooltip label="Tolak">
                                                                    <IconButton aria-label="Reject" icon={<FiX />} size="sm" colorScheme="red" variant="ghost" onClick={() => handleRejectInit(pr)} />
                                                                </Tooltip>
                                                            </>
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

                {/* MODAL CREATE PR */}
                <Modal isOpen={isCreateOpen} onClose={onCreateClose} size="4xl">
                    <ModalOverlay />
                    <ModalContent bg={modalBg} borderWidth="1px" borderColor={borderColor}>
                        <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={borderColor}>Buat Purchase Request Baru</ModalHeader>
                        <ModalCloseButton color={textColor} />
                        <ModalBody>
                            <Grid templateColumns={{ base: '1fr', md: 'repeat(2, 1fr)' }} gap={4} mb={6}>
                                <FormControl>
                                    <FormLabel color={textColor}>Departemen</FormLabel>
                                    <Input value={department} onChange={e => setDepartment(e.target.value)} placeholder="Opsional" bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles} _placeholder={{ color: mutedTextColor }} />
                                </FormControl>
                                <FormControl>
                                    <FormLabel color={textColor}>Tautkan Ke Proyek</FormLabel>
                                    <Select placeholder="-- Pilih Proyek (Opsional) --" value={projectId} onChange={e => setProjectId(e.target.value)} bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles}>
                                        {projects.map(p => (
                                            <option key={p.id} value={p.id}>{p.code} - {p.name}</option>
                                        ))}
                                    </Select>
                                </FormControl>
                            </Grid>

                            <Box border="1px" borderColor={borderColor} rounded="md" p={4} mb={4} bg={infoPanelBg}>
                                <Text fontWeight="bold" mb={4} color={textColor}>Rincian Kebutuhan (Items)</Text>
                                <Table size="sm" variant="unstyled">
                                    <Thead borderBottom="1px" borderColor={borderColor}>
                                        <Tr>
                                            <Th color={mutedTextColor}>Item / Benchmark</Th>
                                            <Th color={mutedTextColor} w="100px">Qty</Th>
                                            <Th color={mutedTextColor} w="180px">Estimasi Harga Satuan</Th>
                                            <Th color={mutedTextColor}>Catatan</Th>
                                            <Th w="50px"></Th>
                                        </Tr>
                                    </Thead>
                                    <Tbody>
                                        {items.map((item, index) => (
                                            <Tr key={index}>
                                                <Td p={1}>
                                                    <Select placeholder="Pilih Item/Barang" value={item.product_id} onChange={e => handleItemChange(index, 'product_id', e.target.value)} bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles}>
                                                        {products.map(p => (
                                                            <option key={p.id} value={p.id}>{p.code} - {p.name}</option>
                                                        ))}
                                                    </Select>
                                                </Td>
                                                <Td p={1}>
                                                    <Input
                                                        type="number"
                                                        min={1}
                                                        value={item.quantity}
                                                        onKeyDown={(e) => {
                                                            // Cegah input huruf 'e', tanda plus/minus, dan titik/koma
                                                            if (['e', 'E', '+', '-', '.', ','].includes(e.key)) {
                                                                e.preventDefault();
                                                            }
                                                        }}
                                                        onChange={e => {
                                                            // Pastikan tidak ada huruf yang lolos (misal dr paste)
                                                            const cleanVal = e.target.value.replace(/[^0-9]/g, '');
                                                            handleItemChange(index, 'quantity', cleanVal);
                                                        }}
                                                        bg={inputBg}
                                                        borderColor={inputBorderColor}
                                                        color={textColor}
                                                        _hover={{ borderColor: inputBorderColor }}
                                                        _focusVisible={fieldFocusStyles}
                                                    />
                                                </Td>
                                                <Td p={1}>
                                                    <CurrencyInput
                                                        value={Number(item.estimated_price) || 0}
                                                        onChange={val => handleItemChange(index, 'estimated_price', val.toString())}
                                                    />
                                                </Td>
                                                <Td p={1}><Input placeholder="Cth: Mendesak" value={item.notes} onChange={e => handleItemChange(index, 'notes', e.target.value)} bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles} _placeholder={{ color: mutedTextColor }} /></Td>
                                                <Td p={1} textAlign="right">
                                                    <IconButton aria-label="Remove" icon={<FiTrash2 />} size="sm" colorScheme="red" variant="ghost" onClick={() => handleRemoveItem(index)} />
                                                </Td>
                                            </Tr>
                                        ))}
                                    </Tbody>
                                </Table>
                                <Button leftIcon={<FiPlus />} size="sm" mt={3} onClick={handleAddItem}>Tambah Item</Button>
                            </Box>

                            <FormControl>
                                <FormLabel color={textColor}>Catatan Umum</FormLabel>
                                <Textarea value={notes} onChange={e => setNotes(e.target.value)} rows={3} bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles} />
                            </FormControl>
                        </ModalBody>
                        <ModalFooter bg={modalFooterBg} borderTop="1px" borderColor={borderColor} borderBottomRadius="md">
                            <Button variant="ghost" mr={3} onClick={onCreateClose}>Batal</Button>
                            <Button colorScheme="blue" isLoading={isSubmitting} onClick={handleCreateSubmit}>Kirim Pengajuan</Button>
                        </ModalFooter>
                    </ModalContent>
                </Modal>

                {/* MODAL VIEW / DETAILS */}
                <Modal isOpen={isViewOpen} onClose={onViewClose} size="3xl">
                    <ModalOverlay />
                    <ModalContent bg={modalBg} borderWidth="1px" borderColor={borderColor}>
                        <ModalHeader display="flex" justifyContent="space-between" alignItems="center" bg={modalHeaderBg} borderBottomWidth="1px" borderColor={borderColor}>
                            <Text>Detail Purchase Request: {selectedRequest?.pr_number}</Text>
                            <Box mr={6}>{selectedRequest && getStatusBadge(selectedRequest.status)}</Box>
                        </ModalHeader>
                        <ModalCloseButton color={textColor} />
                        <ModalBody pb={6}>
                            {selectedRequest && (
                                <Grid templateColumns="repeat(2, 1fr)" gap={4} mb={6} bg={infoPanelBg} p={4} rounded="md">
                                    <Box>
                                        <Text fontSize="sm" color={mutedTextColor}>Pemohon</Text>
                                        <Text fontWeight="medium">{selectedRequest.requester?.first_name} {selectedRequest.requester?.last_name}</Text>
                                    </Box>
                                    <Box>
                                        <Text fontSize="sm" color={mutedTextColor}>Departemen</Text>
                                        <Text fontWeight="medium">{selectedRequest.department || '-'}</Text>
                                    </Box>
                                    <Box>
                                        <Text fontSize="sm" color={mutedTextColor}>Tanggal Pengajuan</Text>
                                        <Text fontWeight="medium">{new Date(selectedRequest.created_at).toLocaleString()}</Text>
                                    </Box>
                                    {selectedRequest.project && (
                                        <Box>
                                            <Text fontSize="sm" color={mutedTextColor}>Dialokasikan ke Proyek</Text>
                                            <Text fontWeight="medium" color="purple.600">{selectedRequest.project.code} - {selectedRequest.project.name}</Text>
                                        </Box>
                                    )}
                                    {selectedRequest.status === 'APPROVED' && selectedRequest.approver && (
                                        <Box>
                                            <Text fontSize="sm" color={mutedTextColor}>Disetujui Oleh</Text>
                                            <Text fontWeight="medium" color="green.600">
                                                {selectedRequest.approver.first_name} {selectedRequest.approver.last_name}
                                                {selectedRequest.approved_at && ` pada ${new Date(selectedRequest.approved_at).toLocaleString()}`}
                                            </Text>
                                        </Box>
                                    )}
                                </Grid>
                            )}

                            <Text fontWeight="bold" mb={3}>Rincian Item</Text>
                            <Table variant="striped" size="sm" mb={4}>
                                <Thead>
                                    <Tr>
                                        <Th>Item (Benchmark)</Th>
                                        <Th isNumeric>Qty</Th>
                                        <Th isNumeric>Est. Harga Satuan</Th>
                                        <Th isNumeric>Total Est.</Th>
                                    </Tr>
                                </Thead>
                                <Tbody>
                                    {selectedRequest?.items?.map(it => (
                                        <Tr key={it.id}>
                                            <Td>
                                                <Text fontWeight="medium">{it.product?.name}</Text>
                                                {it.notes && <Text fontSize="xs" color={mutedTextColor}>{it.notes}</Text>}
                                            </Td>
                                            <Td isNumeric>{it.quantity}</Td>
                                            <Td isNumeric>{formatCurrency(it.estimated_price)}</Td>
                                            <Td isNumeric fontWeight="semibold">{formatCurrency(it.total_estimated_price)}</Td>
                                        </Tr>
                                    ))}
                                    <Tr>
                                        <Td colSpan={3} textAlign="right" fontWeight="bold">Total Pagu Request:</Td>
                                        <Td isNumeric fontWeight="bold" fontSize="md" color="blue.600">{formatCurrency(selectedRequest?.total_estimated_amount || 0)}</Td>
                                    </Tr>
                                </Tbody>
                            </Table>

                            {selectedRequest?.notes && (
                                <Box mt={4}>
                                    <Text fontSize="sm" fontWeight="bold">Catatan:</Text>
                                    <Text mt={1} p={3} bg={notePanelBg} color={notePanelColor} rounded="md" fontSize="sm">{selectedRequest.notes}</Text>
                                </Box>
                            )}
                        </ModalBody>
                        <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={borderColor}>
                            {canApprove('purchase_requests') && selectedRequest?.status === 'PENDING_APPROVAL' && (
                                <Flex gap={2}>
                                    <Button colorScheme="red" variant="outline" onClick={() => handleRejectInit(selectedRequest)}>Tolak Request</Button>
                                    <Button colorScheme="green" onClick={() => { handleApprove(selectedRequest.id); onViewClose(); }}>Setujui Target Pagu</Button>
                                </Flex>
                            )}
                        </ModalFooter>
                    </ModalContent>
                </Modal>

                {/* MODAL REJECT */}
                <Modal isOpen={isRejectOpen} onClose={onRejectClose}>
                    <ModalOverlay />
                    <ModalContent bg={modalBg} borderWidth="1px" borderColor={borderColor}>
                        <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={borderColor}>Alasan Penolakan</ModalHeader>
                        <ModalCloseButton color={textColor} />
                        <ModalBody>
                            <Text mb={4} color={textColor}>Mengapa Anda menolak Purchase Request {selectedRequest?.pr_number}?</Text>
                            <Textarea placeholder="Alasan penolakan (misal: Over-budget, item tidak diperlukan)" value={rejectReason} onChange={e => setRejectReason(e.target.value)} rows={4} bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles} _placeholder={{ color: mutedTextColor }} />
                        </ModalBody>
                        <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={borderColor}>
                            <Button variant="ghost" mr={3} onClick={onRejectClose}>Batal</Button>
                            <Button colorScheme="red" onClick={handleRejectSubmit} isLoading={isSubmitting}>Kirim Penolakan</Button>
                        </ModalFooter>
                    </ModalContent>
                </Modal>

            </Layout>
        </ProtectedModule>
    );
}
