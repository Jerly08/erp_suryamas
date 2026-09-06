'use client';

import React, { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  AlertDescription,
  AlertIcon,
  Badge,
  Box,
  Button,
  Divider,
  Flex,
  FormControl,
  FormLabel,
  HStack,
  Icon,
  IconButton,
  Input,
  Select,
  Spinner,
  Text,
  Textarea,
  useColorModeValue,
  useToast,
  VStack,
} from '@chakra-ui/react';
import {
  FiDownload,
  FiFileText,
  FiPaperclip,
  FiRefreshCw,
  FiUpload,
  FiX,
} from 'react-icons/fi';
import paymentService, {
  PaymentDocument,
  PaymentDocumentType,
} from '@/services/paymentService';

export const PAYMENT_DOCUMENT_ACCEPT =
  '.pdf,.doc,.docx,.xls,.xlsx,.xlsm,.csv,.txt,.jpg,.jpeg,.png,.webp';
export const MAX_PAYMENT_DOCUMENT_SIZE_MB = 50;
export const MAX_PAYMENT_DOCUMENT_SIZE = MAX_PAYMENT_DOCUMENT_SIZE_MB * 1024 * 1024;

export const PAYMENT_DOCUMENT_TYPE_OPTIONS: Array<{
  value: PaymentDocumentType;
  label: string;
}> = [
  { value: 'VENDOR_INVOICE', label: 'Invoice Vendor' },
  { value: 'PAYMENT_PROOF', label: 'Bukti Transfer' },
  { value: 'TAX_INVOICE', label: 'Faktur Pajak' },
  { value: 'SUPPORTING_DOCUMENT', label: 'Dokumen Pendukung' },
  { value: 'OTHER', label: 'Lainnya' },
];

export interface StagedPaymentDocument {
  clientId: string;
  file: File;
  documentType: PaymentDocumentType | string;
  referenceNumber: string;
  notes: string;
  uploadState?: 'PENDING' | 'UPLOADING' | 'FAILED';
  uploadError?: string;
}

type StagedDocumentSetter = React.Dispatch<React.SetStateAction<StagedPaymentDocument[]>>;

const formatFileSize = (size?: number) => {
  if (!size || size < 0) return '-';
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
};

const formatDocumentDateTime = (value?: string) => {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleString('id-ID', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    timeZone: 'Asia/Jakarta',
    timeZoneName: 'short',
  });
};

const getUploaderName = (document: PaymentDocument) => {
  const uploader = document.uploader;
  if (!uploader) return '';
  const fullName = `${uploader.first_name || ''} ${uploader.last_name || ''}`.trim();
  return fullName || uploader.username || uploader.email || '';
};

const getErrorMessage = (error: unknown) => {
  const uploadError = error as {
    message?: string;
    response?: { data?: { error?: string; message?: string; details?: string } };
  };
  return uploadError.response?.data?.error ||
    uploadError.response?.data?.message ||
    uploadError.response?.data?.details ||
    uploadError.message ||
    'Upload dokumen gagal. Coba lagi.';
};

const validateDocument = (file: File) => {
  const extension = `.${file.name.split('.').pop()?.toLowerCase() || ''}`;
  const allowedExtensions = PAYMENT_DOCUMENT_ACCEPT.split(',');
  if (!allowedExtensions.includes(extension)) {
    return 'Format dokumen belum didukung. Gunakan PDF, Word, Excel, CSV, TXT, JPG, PNG, atau WebP.';
  }
  if (file.size > MAX_PAYMENT_DOCUMENT_SIZE) {
    return `Ukuran dokumen terlalu besar. Maksimal ${MAX_PAYMENT_DOCUMENT_SIZE_MB}MB per file.`;
  }
  return null;
};

export const createStagedPaymentDocuments = (
  files: File[],
): StagedPaymentDocument[] => files.map((file, index) => ({
  clientId: `${file.name}-${file.size}-${file.lastModified}-${index}`,
  file,
  documentType: 'SUPPORTING_DOCUMENT',
  referenceNumber: '',
  notes: '',
  uploadState: 'PENDING',
}));

interface PaymentDocumentStagingPanelProps {
  documents: StagedPaymentDocument[];
  onDocumentsChange: StagedDocumentSetter;
  isDisabled?: boolean;
  retryPaymentCode?: string;
}

/**
 * Client-side staging only. Nothing is sent until a payment has a durable ID,
 * so an interrupted attachment upload can safely be retried without creating
 * another payment.
 */
export const PaymentDocumentStagingPanel: React.FC<PaymentDocumentStagingPanelProps> = ({
  documents,
  onDocumentsChange,
  isDisabled = false,
  retryPaymentCode,
}) => {
  const toast = useToast();
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const mutedTextColor = useColorModeValue('gray.500', 'gray.400');

  const handleFilesChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files || []);
    event.target.value = '';
    if (files.length === 0) return;

    const accepted: File[] = [];
    const rejected: string[] = [];
    files.forEach((file) => {
      const error = validateDocument(file);
      if (error) {
        rejected.push(`${file.name}: ${error}`);
        return;
      }
      const duplicate = documents.some(
        (document) =>
          document.file.name === file.name &&
          document.file.size === file.size &&
          document.file.lastModified === file.lastModified,
      );
      if (duplicate) {
        rejected.push(`${file.name}: file sudah dipilih.`);
        return;
      }
      accepted.push(file);
    });

    if (accepted.length > 0) {
      onDocumentsChange((current) => [
        ...current,
        ...createStagedPaymentDocuments(accepted),
      ]);
    }
    if (rejected.length > 0) {
      toast({
        title: 'Sebagian dokumen tidak ditambahkan',
        description: rejected.join(' '),
        status: 'warning',
        duration: 7000,
        isClosable: true,
      });
    }
  };

  const updateDocument = (
    clientId: string,
    patch: Partial<Pick<StagedPaymentDocument, 'documentType' | 'referenceNumber' | 'notes' | 'uploadState' | 'uploadError'>>,
  ) => {
    onDocumentsChange((current) => current.map((document) =>
      document.clientId === clientId ? { ...document, ...patch } : document,
    ));
  };

  return (
    <Box borderWidth="1px" borderColor={borderColor} borderRadius="md" p={4}>
      <Flex
        justify="space-between"
        align={{ base: 'flex-start', md: 'center' }}
        direction={{ base: 'column', md: 'row' }}
        gap={3}
      >
        <Box>
          <Text fontWeight="semibold">Dokumen Pendukung</Text>
          <Text fontSize="sm" color={mutedTextColor}>
            Upload invoice, bukti transfer, faktur pajak, atau dokumen pendukung lainnya. Maksimal {MAX_PAYMENT_DOCUMENT_SIZE_MB}MB per file.
          </Text>
        </Box>
        <Button
          as="label"
          leftIcon={<FiPaperclip />}
          variant="outline"
          colorScheme="blue"
          cursor={isDisabled ? 'not-allowed' : 'pointer'}
          isDisabled={isDisabled}
          flexShrink={0}
        >
          Pilih Dokumen
          <Input
            type="file"
            multiple
            accept={PAYMENT_DOCUMENT_ACCEPT}
            display="none"
            disabled={isDisabled}
            onChange={handleFilesChange}
          />
        </Button>
      </Flex>

      {retryPaymentCode && (
        <Alert status="warning" mt={3} borderRadius="md">
          <AlertIcon />
          <AlertDescription fontSize="sm">
            Pembayaran <strong>{retryPaymentCode}</strong> sudah tersimpan. Dokumen yang gagal dapat diunggah ulang tanpa membuat pembayaran baru.
          </AlertDescription>
        </Alert>
      )}

      {documents.length > 0 && (
        <VStack align="stretch" spacing={0} mt={4} divider={<Divider borderColor={borderColor} />}>
          {documents.map((document) => (
            <Box key={document.clientId} py={3}>
              <Flex justify="space-between" align="flex-start" gap={3}>
                <HStack minW={0} spacing={2} align="flex-start">
                  {document.uploadState === 'UPLOADING' ? (
                    <Spinner size="sm" mt={1} color="blue.500" />
                  ) : (
                    <Icon as={FiFileText} color={document.uploadState === 'FAILED' ? 'red.500' : 'blue.500'} mt={1} />
                  )}
                  <Box minW={0}>
                    <Text fontSize="sm" fontWeight="medium" noOfLines={1}>{document.file.name}</Text>
                    <Text fontSize="xs" color={mutedTextColor}>{formatFileSize(document.file.size)}</Text>
                  </Box>
                </HStack>
                <HStack>
                  {document.uploadState === 'FAILED' && <Badge colorScheme="red">Gagal</Badge>}
                  <IconButton
                    aria-label="Hapus dokumen"
                    icon={<FiX />}
                    size="sm"
                    variant="ghost"
                    colorScheme="red"
                    isDisabled={isDisabled || document.uploadState === 'UPLOADING'}
                    onClick={() => onDocumentsChange((current) => current.filter((item) => item.clientId !== document.clientId))}
                  />
                </HStack>
              </Flex>
              <Flex mt={3} gap={3} direction={{ base: 'column', md: 'row' }}>
                <FormControl maxW={{ md: '230px' }}>
                  <FormLabel fontSize="xs" mb={1}>Jenis Dokumen</FormLabel>
                  <Select
                    size="sm"
                    value={document.documentType}
                    isDisabled={isDisabled || document.uploadState === 'UPLOADING'}
                    onChange={(event) => updateDocument(document.clientId, {
                      documentType: event.target.value,
                      uploadState: document.uploadState === 'FAILED' ? 'PENDING' : document.uploadState,
                      uploadError: undefined,
                    })}
                  >
                    {PAYMENT_DOCUMENT_TYPE_OPTIONS.map((option) => (
                      <option key={option.value} value={option.value}>{option.label}</option>
                    ))}
                  </Select>
                </FormControl>
                <FormControl maxW={{ md: '260px' }}>
                  <FormLabel fontSize="xs" mb={1}>Nomor Referensi</FormLabel>
                  <Input
                    size="sm"
                    value={document.referenceNumber}
                    isDisabled={isDisabled || document.uploadState === 'UPLOADING'}
                    placeholder="Contoh: INV-001 / nomor faktur"
                    maxLength={100}
                    onChange={(event) => updateDocument(document.clientId, {
                      referenceNumber: event.target.value,
                      uploadState: document.uploadState === 'FAILED' ? 'PENDING' : document.uploadState,
                      uploadError: undefined,
                    })}
                  />
                </FormControl>
                <FormControl>
                  <FormLabel fontSize="xs" mb={1}>Catatan Dokumen</FormLabel>
                  <Textarea
                    size="sm"
                    rows={1}
                    resize="vertical"
                    value={document.notes}
                    isDisabled={isDisabled || document.uploadState === 'UPLOADING'}
                    placeholder="Contoh: invoice periode Januari 2026"
                    onChange={(event) => updateDocument(document.clientId, {
                      notes: event.target.value,
                      uploadState: document.uploadState === 'FAILED' ? 'PENDING' : document.uploadState,
                      uploadError: undefined,
                    })}
                  />
                </FormControl>
              </Flex>
              {document.uploadError && (
                <Text color="red.500" fontSize="xs" mt={2}>{document.uploadError}</Text>
              )}
            </Box>
          ))}
        </VStack>
      )}
    </Box>
  );
};

interface PaymentDocumentsPanelProps {
  paymentId: number;
  initialDocuments?: PaymentDocument[];
  isActive?: boolean;
  canAddDocuments?: boolean;
  addDocumentsDisabledMessage?: string;
}

/**
 * Persistent document history used from payment details. It reloads from the
 * API when opened rather than assuming the payment list has preloaded files.
 */
const PaymentDocumentsPanel: React.FC<PaymentDocumentsPanelProps> = ({
  paymentId,
  initialDocuments,
  isActive = true,
  canAddDocuments = true,
  addDocumentsDisabledMessage,
}) => {
  const toast = useToast();
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const mutedTextColor = useColorModeValue('gray.500', 'gray.400');
  const [documents, setDocuments] = useState<PaymentDocument[]>(initialDocuments || []);
  const [stagedDocuments, setStagedDocuments] = useState<StagedPaymentDocument[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isUploading, setIsUploading] = useState(false);

  const loadDocuments = useCallback(async () => {
    if (!paymentId) return;
    setIsLoading(true);
    try {
      setDocuments(await paymentService.listPaymentDocuments(paymentId));
    } catch (error) {
      console.error('Tidak dapat memuat dokumen pembayaran:', error);
      toast({
        title: 'Dokumen pembayaran belum dapat dimuat',
        description: getErrorMessage(error),
        status: 'warning',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setIsLoading(false);
    }
  }, [paymentId, toast]);

  useEffect(() => {
    if (initialDocuments) {
      setDocuments(initialDocuments);
    }
  }, [initialDocuments, paymentId]);

  useEffect(() => {
    if (isActive && paymentId) {
      void loadDocuments();
    }
  }, [isActive, loadDocuments, paymentId]);

  const uploadStagedDocuments = async () => {
    const uploadable = stagedDocuments.filter((document) => document.uploadState !== 'UPLOADING');
    if (uploadable.length === 0) return;

    setIsUploading(true);
    const uploadedIds: string[] = [];
    let failedCount = 0;
    for (const document of uploadable) {
      setStagedDocuments((current) => current.map((item) => item.clientId === document.clientId
        ? { ...item, uploadState: 'UPLOADING', uploadError: undefined }
        : item,
      ));
      try {
        await paymentService.uploadPaymentDocument(paymentId, document.file, {
          documentType: document.documentType,
          referenceNumber: document.referenceNumber,
          notes: document.notes,
        });
        uploadedIds.push(document.clientId);
      } catch (error) {
        failedCount += 1;
        setStagedDocuments((current) => current.map((item) => item.clientId === document.clientId
          ? { ...item, uploadState: 'FAILED', uploadError: getErrorMessage(error) }
          : item,
        ));
      }
    }
    setStagedDocuments((current) => current.filter((document) => !uploadedIds.includes(document.clientId)));
    await loadDocuments();
    setIsUploading(false);

    if (failedCount > 0) {
      toast({
        title: 'Sebagian dokumen belum terunggah',
        description: 'Pembayaran tidak dibuat ulang. Periksa file yang gagal lalu klik Unggah Dokumen lagi.',
        status: 'warning',
        duration: 7000,
        isClosable: true,
      });
      return;
    }
    toast({ title: 'Dokumen pendukung berhasil diunggah', status: 'success', duration: 3500 });
  };

  const handleDownload = async (document: PaymentDocument) => {
    try {
      await paymentService.downloadPaymentDocument(document.id, document.file_name || 'dokumen-pembayaran');
    } catch (error) {
      console.error('Tidak dapat mengunduh dokumen pembayaran:', error);
      toast({
        title: 'Dokumen gagal diunduh',
        description: getErrorMessage(error),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    }
  };

  return (
    <Box>
      <Flex align="center" justify="space-between" gap={3} mb={3}>
        <Box>
          <Text fontWeight="bold" fontSize="lg">Dokumen Pendukung</Text>
          <Text fontSize="sm" color={mutedTextColor}>Riwayat lampiran pembayaran disimpan terpisah dari nominal dan jurnal.</Text>
        </Box>
        <IconButton
          aria-label="Muat ulang dokumen pembayaran"
          icon={<FiRefreshCw />}
          size="sm"
          variant="ghost"
          onClick={() => void loadDocuments()}
          isLoading={isLoading}
        />
      </Flex>

      {isLoading && documents.length === 0 ? (
        <Flex justify="center" py={5}><Spinner /></Flex>
      ) : documents.length === 0 ? (
        <Text color={mutedTextColor} fontSize="sm" py={2}>Belum ada dokumen pendukung pada pembayaran ini.</Text>
      ) : (
        <VStack align="stretch" spacing={0} borderWidth="1px" borderColor={borderColor} borderRadius="md" px={3} divider={<Divider borderColor={borderColor} />}>
          {documents.map((document) => {
            const uploaderName = getUploaderName(document);
            const uploadedAt = formatDocumentDateTime(document.created_at);
            return (
              <Flex key={document.id} py={3} align="flex-start" justify="space-between" gap={3}>
                <HStack align="flex-start" spacing={2} minW={0}>
                  <Icon as={FiFileText} color="blue.500" mt={1} />
                  <Box minW={0}>
                    <HStack spacing={2} flexWrap="wrap">
                      <Text fontSize="sm" fontWeight="medium" noOfLines={1}>{document.file_name}</Text>
                      <Badge colorScheme="blue" fontSize="xs">
                        {PAYMENT_DOCUMENT_TYPE_OPTIONS.find((option) => option.value === document.document_type)?.label || document.document_type || 'Dokumen'}
                      </Badge>
                      {document.status && document.status !== 'ACTIVE' && (
                        <Badge colorScheme="gray" fontSize="xs">{document.status}</Badge>
                      )}
                    </HStack>
                    <Text fontSize="xs" color={mutedTextColor} mt={1}>
                      {formatFileSize(document.file_size)}
                      {uploaderName ? ` • diunggah oleh ${uploaderName}` : ''}
                      {uploadedAt ? ` • ${uploadedAt}` : ''}
                    </Text>
                    {document.reference_number && (
                      <Text fontSize="xs" color={mutedTextColor} mt={1}>
                        Referensi: {document.reference_number}
                      </Text>
                    )}
                    {document.notes && <Text fontSize="xs" color={mutedTextColor} mt={1}>{document.notes}</Text>}
                  </Box>
                </HStack>
                <IconButton
                  aria-label={`Unduh ${document.file_name}`}
                  icon={<FiDownload />}
                  size="sm"
                  variant="outline"
                  onClick={() => void handleDownload(document)}
                />
              </Flex>
            );
          })}
        </VStack>
      )}

      {canAddDocuments ? (
        <Box mt={4}>
          <PaymentDocumentStagingPanel
            documents={stagedDocuments}
            onDocumentsChange={setStagedDocuments}
            isDisabled={isUploading}
          />
          {stagedDocuments.length > 0 && (
            <Flex justify="flex-end" mt={3}>
              <Button
                leftIcon={<FiUpload />}
                colorScheme="blue"
                onClick={() => void uploadStagedDocuments()}
                isLoading={isUploading}
                loadingText="Mengunggah"
              >
                Unggah Dokumen
              </Button>
            </Flex>
          )}
        </Box>
      ) : (
        <Alert status="info" mt={4} borderRadius="md">
          <AlertIcon />
          <AlertDescription fontSize="sm">
            {addDocumentsDisabledMessage || 'Dokumen baru tidak dapat ditambahkan karena pembayaran ini sudah dibatalkan, ditolak, atau dibalik.'}
          </AlertDescription>
        </Alert>
      )}
    </Box>
  );
};

export default PaymentDocumentsPanel;
