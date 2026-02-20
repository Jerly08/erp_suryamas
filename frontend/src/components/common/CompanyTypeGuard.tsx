import React, { useEffect, useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import CompanyTypeSelection from '@/components/onboarding/CompanyTypeSelection';
import SimpleLayout from '@/components/layout/SimpleLayout';
import { Flex, Spinner, VStack, Text } from '@chakra-ui/react';

interface CompanyTypeGuardProps {
    children: React.ReactNode;
}

const CompanyTypeGuard: React.FC<CompanyTypeGuardProps> = ({ children }) => {
    const { user, isLoading, checkAuth } = useAuth();
    const [isVerifying, setIsVerifying] = useState(true);

    useEffect(() => {
        if (!isLoading) {
            setIsVerifying(false);
        }
    }, [isLoading]);

    const handleSuccess = async () => {
        setIsVerifying(true);
        // Refresh auth state to get the new company_type
        await checkAuth();
        setIsVerifying(false);
    };

    if (isLoading || isVerifying) {
        return (
            <SimpleLayout>
                <Flex justify="center" align="center" minH="60vh">
                    <VStack spacing={4}>
                        <Spinner size="xl" color="blue.500" thickness="4px" />
                        <Text>Verifying company configuration...</Text>
                    </VStack>
                </Flex>
            </SimpleLayout>
        );
    }

    // If user is logged in but no company_type, show selection
    // Note: user?.role === 'admin' check is NOT needed here because any user 
    // trying to create transactions needs the company type to be set.
    // However, only admins can set it (handled by backend/component permissions).
    // If a non-admin sees this, CompanyTypeSelection might need to handle the "Contact Admin" case.
    // But based on user request "admin harus memilih company type dulu", implying admin does it.
    // If a standard employee encounters this, they should probably see "Contact Admin".

    if (user && !user.company_type) {
        // Check if user is allowed to set company type (Admin only)
        if (user.role === 'admin') {
            return (
                <SimpleLayout>
                    <CompanyTypeSelection onSuccess={handleSuccess} />
                </SimpleLayout>
            );
        } else {
            // Non-admin users see a blocking message
            return (
                <SimpleLayout>
                    <Flex justify="center" align="center" minH="60vh" direction="column">
                        <Text fontSize="xl" fontWeight="bold" mb={2}>Company Configuration Required</Text>
                        <Text color="gray.600">Please contact your administrator to select a Company Type before proceeding.</Text>
                    </Flex>
                </SimpleLayout>
            );
        }
    }

    return <>{children}</>;
};

export default CompanyTypeGuard;
