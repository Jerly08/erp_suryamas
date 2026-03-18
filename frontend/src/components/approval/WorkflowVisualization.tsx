'use client';

import React, { useState, useEffect } from 'react';
import {
  Box,
  VStack,
  HStack,
  Text,
  Card,
  CardBody,
  Badge,
  Icon,
  Flex,
  Circle,
  Progress,
  Tooltip,
  useColorModeValue,
} from '@chakra-ui/react';
import {
  FiUser,
  FiDollarSign,
  FiCheckCircle,
  FiXCircle,
  FiClock,
  FiArrowRight,
  FiArrowDown,
  FiUsers,
} from 'react-icons/fi';

interface ApprovalStep {
  id: number;
  step_order: number;
  step_name: string;
  approver_role: string;
  is_optional: boolean;
  is_parallel: boolean;
  time_limit: number;
}

interface ApprovalAction {
  id: number;
  step_id: number;
  status: string;
  approver_id?: number;
  action_date?: string;
  is_active: boolean;
  comments?: string;
  approver?: {
    first_name: string;
    last_name: string;
  };
}

interface WorkflowVisualizationProps {
  steps: ApprovalStep[];
  actions: ApprovalAction[];
  currentStatus: string;
}

const WorkflowVisualization: React.FC<WorkflowVisualizationProps> = ({
  steps,
  actions,
  currentStatus,
}) => {
  const bgColor = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  
  const getStepStatus = (step: ApprovalStep) => {
    const action = actions.find(a => a.step_id === step.id);
    if (!action) return 'pending';
    
    if (action.is_active && action.status === 'PENDING') return 'active';
    if (action.status === 'APPROVED') return 'approved';
    if (action.status === 'REJECTED') return 'rejected';
    if (action.status === 'SKIPPED') return 'skipped';
    return 'pending';
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'approved': return 'green';
      case 'rejected': return 'red';
      case 'active': return 'blue';
      case 'skipped': return 'orange';
      default: return 'gray';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'approved': return FiCheckCircle;
      case 'rejected': return FiXCircle;
      case 'active': return FiClock;
      case 'skipped': return FiArrowRight;
      default: return FiUser;
    }
  };

  const getRoleIcon = (role: string) => {
    switch (role.toLowerCase()) {
      case 'finance': return FiDollarSign;
      case 'director': return FiUsers;
      default: return FiUser;
    }
  };

  const sortedSteps = [...steps].sort((a, b) => a.step_order - b.step_order);
  
  // Group parallel steps
  const stepGroups: ApprovalStep[][] = [];
  let currentGroup: ApprovalStep[] = [];
  let currentOrder = -1;

  sortedSteps.forEach(step => {
    if (step.step_order !== currentOrder) {
      if (currentGroup.length > 0) {
        stepGroups.push(currentGroup);
      }
      currentGroup = [step];
      currentOrder = step.step_order;
    } else if (step.is_parallel) {
      currentGroup.push(step);
    } else {
      if (currentGroup.length > 0) {
        stepGroups.push(currentGroup);
      }
      currentGroup = [step];
    }
  });
  
  if (currentGroup.length > 0) {
    stepGroups.push(currentGroup);
  }

  const calculateProgress = () => {
    const totalSteps = steps.length;
    const completedSteps = actions.filter(a => 
      a.status === 'APPROVED' || a.status === 'REJECTED'
    ).length;
    return totalSteps > 0 ? (completedSteps / totalSteps) * 100 : 0;
  };

  return (
    <Box w="full" p={4}>
      <VStack spacing={6} align="stretch">
        {/* Progress Header */}
        <Card>
          <CardBody>
            <VStack spacing={4}>
              <HStack justify="space-between" w="full">
                <Text fontSize="lg" fontWeight="bold">
                  Approval Workflow Progress
                </Text>
                <Badge
                  colorScheme={getStatusColor(currentStatus.toLowerCase())}
                  variant="subtle"
                  px={3}
                  py={1}
                  borderRadius="full"
                >
                  {currentStatus}
                </Badge>
              </HStack>
              <Progress
                value={calculateProgress()}
                colorScheme="blue"
                w="full"
                size="lg"
                borderRadius="full"
              />
              <Text fontSize="sm" color="gray.600">
                {Math.round(calculateProgress())}% Complete
              </Text>
            </VStack>
          </CardBody>
        </Card>

        {/* Workflow Visualization */}
        <Card>
          <CardBody>
            <VStack spacing={8} align="center">
              {stepGroups.map((group, groupIndex) => (
                <Box key={groupIndex} position="relative">
                  {/* Step Group */}
                  {group.length === 1 ? (
                    // Single step
                    <StepCard step={group[0]} actions={actions} />
                  ) : (
                    // Parallel steps
                    <VStack spacing={2} align="center">
                      <Text fontSize="sm" color="gray.500" fontWeight="medium">
                        Parallel Approval
                      </Text>
                      <HStack spacing={8}>
                        {group.map(step => (
                          <StepCard key={step.id} step={step} actions={actions} />
                        ))}
                      </HStack>
                    </VStack>
                  )}

                  {/* Arrow to next group */}
                  {groupIndex < stepGroups.length - 1 && (
                    <Flex justify="center" mt={4} mb={4}>
                      <Icon 
                        as={FiArrowDown} 
                        boxSize={6} 
                        color="gray.400"
                        animation="bounce 2s infinite"
                      />
                    </Flex>
                  )}
                </Box>
              ))}
            </VStack>
          </CardBody>
        </Card>
      </VStack>
    </Box>
  );
};

// Individual Step Card Component
interface StepCardProps {
  step: ApprovalStep;
  actions: ApprovalAction[];
}

const StepCard: React.FC<StepCardProps> = ({ step, actions }) => {
  const bgColor = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');

  function getStepStatus(stepData: ApprovalStep) {
    const actionItem = actions.find(a => a.step_id === stepData.id);
    if (!actionItem) return 'pending';

    if (actionItem.is_active && actionItem.status === 'PENDING') return 'active';
    if (actionItem.status === 'APPROVED') return 'approved';
    if (actionItem.status === 'REJECTED') return 'rejected';
    if (actionItem.status === 'SKIPPED') return 'skipped';
    return 'pending';
  }

  function getStatusColor(statusValue: string) {
    switch (statusValue) {
      case 'approved': return 'green';
      case 'rejected': return 'red';
      case 'active': return 'blue';
      case 'skipped': return 'orange';
      default: return 'gray';
    }
  }

  function getStatusIcon(statusValue: string) {
    switch (statusValue) {
      case 'approved': return FiCheckCircle;
      case 'rejected': return FiXCircle;
      case 'active': return FiClock;
      case 'skipped': return FiArrowRight;
      default: return FiUser;
    }
  }

  function getRoleIcon(role: string) {
    switch (role.toLowerCase()) {
      case 'finance': return FiDollarSign;
      case 'director': return FiUsers;
      default: return FiUser;
    }
  }

  const status = getStepStatus(step);
  const action = actions.find(a => a.step_id === step.id);
  const statusColor = getStatusColor(status);
  const StatusIcon = getStatusIcon(status);
  const RoleIcon = getRoleIcon(step.approver_role);

  return (
    <Tooltip
      label={
        action?.comments || 
        `${step.step_name} - ${step.approver_role} approval required`
      }
      placement="top"
    >
      <Card
        variant="outline"
        borderColor={`${statusColor}.200`}
        borderWidth={status === 'active' ? 3 : 1}
        bg={status === 'active' ? `${statusColor}.50` : bgColor}
        minW="200px"
        cursor="pointer"
        _hover={{ transform: 'translateY(-2px)', shadow: 'lg' }}
        transition="all 0.2s"
      >
        <CardBody p={4}>
          <VStack spacing={3}>
            {/* Status and Role Icons */}
            <HStack spacing={3} w="full" justify="center">
              <Circle
                size="40px"
                bg={`${statusColor}.100`}
                border={`2px solid`}
                borderColor={`${statusColor}.300`}
              >
                <Icon as={StatusIcon} color={`${statusColor}.600`} boxSize={5} />
              </Circle>
              <Circle
                size="30px"
                bg="gray.100"
                border={`1px solid`}
                borderColor="gray.300"
              >
                <Icon as={RoleIcon} color="gray.600" boxSize={4} />
              </Circle>
            </HStack>

            {/* Step Information */}
            <VStack spacing={1} textAlign="center">
              <Text fontSize="sm" fontWeight="bold">
                {step.step_name}
              </Text>
              <Text fontSize="xs" color="gray.500" textTransform="capitalize">
                {step.approver_role}
              </Text>
              {step.is_optional && (
                <Badge colorScheme="gray" size="sm">
                  Optional
                </Badge>
              )}
            </VStack>

            {/* Action Details */}
            {action && action.approver && (
              <Text fontSize="xs" color="gray.600" textAlign="center">
                {action.approver.first_name} {action.approver.last_name}
              </Text>
            )}
            
            {action?.action_date && (
              <Text fontSize="xs" color="gray.500">
                {new Date(action.action_date).toLocaleDateString()}
              </Text>
            )}
          </VStack>
        </CardBody>
      </Card>
    </Tooltip>
  );
};

export default WorkflowVisualization;
