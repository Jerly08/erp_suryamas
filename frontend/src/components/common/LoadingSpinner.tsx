'use client';

import React from 'react';
import { Box, Flex, Text, VStack, useColorModeValue } from '@chakra-ui/react';
import { keyframes } from '@emotion/react';

interface LoadingSpinnerProps {
  size?: 'sm' | 'md' | 'lg' | 'xl';
  message?: string;
  showMessage?: boolean;
  color?: string;
}

const spin = keyframes`
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
`;

const pulse = keyframes`
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
`;

const shimmer = keyframes`
  0% { opacity: 0.45; }
  50% { opacity: 1; }
  100% { opacity: 0.45; }
`;

const LoadingSpinner: React.FC<LoadingSpinnerProps> = ({
  size = 'md',
  message = 'Loading...',
  showMessage = true,
  color,
}) => {
  const getSizeProps = () => {
    switch (size) {
      case 'sm':
        return { boxSize: '20px', borderWidth: '2px', textSize: 'xs' };
      case 'md':
        return { boxSize: '32px', borderWidth: '3px', textSize: 'sm' };
      case 'lg':
        return { boxSize: '40px', borderWidth: '4px', textSize: 'md' };
      case 'xl':
        return { boxSize: '56px', borderWidth: '4px', textSize: 'lg' };
      default:
        return { boxSize: '32px', borderWidth: '3px', textSize: 'sm' };
    }
  };

  const { boxSize, borderWidth, textSize } = getSizeProps();
  const defaultSpinnerColor = useColorModeValue('#2196f3', '#4493f8');
  const messageColor = useColorModeValue('gray.600', '#8d96a0');
  const spinnerColor = color || defaultSpinnerColor;

  return (
    <Flex
      direction="column"
      align="center"
      justify="center"
      minH="200px"
      gap={4}
    >
      <VStack spacing={4}>
        {/* Main Spinner */}
        <Box
          width={boxSize}
          height={boxSize}
          border={borderWidth}
          borderStyle="solid"
          borderColor="transparent"
          borderTopColor={spinnerColor}
          borderRadius="50%"
          animation={`${spin} 1s linear infinite`}
          willChange="transform"
        />
        
        {/* Message */}
        {showMessage && (
          <Text
            fontSize={textSize}
            color={messageColor}
            animation={`${pulse} 2s ease-in-out infinite`}
            fontWeight="medium"
            letterSpacing="wide"
          >
            {message}
          </Text>
        )}
      </VStack>
    </Flex>
  );
};

// Loading skeleton for cards
export const LoadingSkeleton: React.FC<{ height?: string; count?: number }> = ({ 
  height = '20px', 
  count = 3 
}) => {
  const skeletonBg = useColorModeValue('gray.200', 'gray.700');
  
  return (
    <VStack spacing={3} align="stretch">
      {Array.from({ length: count }).map((_, index) => (
        <Box
          key={index}
          height={height}
          bg={skeletonBg}
          borderRadius="md"
          animation={`${shimmer} 2s ease-in-out infinite`}
          style={{ animationDelay: `${index * 0.1}s` }}
        />
      ))}
    </VStack>
  );
};

export default LoadingSpinner;
