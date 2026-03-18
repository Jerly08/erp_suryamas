import React, { useState } from 'react';
import { Box, Button, Typography, Card, CardContent, Grid, Alert, CircularProgress } from '@mui/material';
import AccountBalanceIcon from '@mui/icons-material/AccountBalance';
import AddIcon from '@mui/icons-material/Add';
import ReceiptIcon from '@mui/icons-material/Receipt';
import DataArrayIcon from '@mui/icons-material/DataArray';
import { sampleDataService } from '../../services/sampleDataService';

interface BalanceSheetEmptyStateProps {
  onRefresh: () => void;
}

const BalanceSheetEmptyState: React.FC<BalanceSheetEmptyStateProps> = ({ onRefresh }) => {
  const [isGenerating, setIsGenerating] = useState(false);
  const [result, setResult] = useState<{
    success?: boolean;
    message?: string;
  } | null>(null);

  const handleGenerateSampleData = async () => {
    setIsGenerating(true);
    setResult(null);
    
    try {
      // Check if sample data already exists
      const exists = await sampleDataService.checkSampleDataExists();
      
      let response;
      if (exists) {
        // If sample data exists, just show a message
        response = {
          success: true,
          message: 'Sample data already exists. Refreshing the balance sheet view...'
        };
      } else {
        // If no sample data, create it
        response = await sampleDataService.createSampleBalanceSheetData();
      }
      
      setResult(response);
      
      // Wait a moment and then refresh the balance sheet
      if (response.success) {
        setTimeout(() => {
          onRefresh();
        }, 1500);
      }
    } catch (error) {
      setResult({
        success: false,
        message: `Error generating sample data: ${error instanceof Error ? error.message : 'Unknown error'}`
      });
    } finally {
      setIsGenerating(false);
    }
  };

  return (
    <Box sx={{ p: 3, textAlign: 'center', maxWidth: 800, mx: 'auto' }}>
      <AccountBalanceIcon sx={{ fontSize: 64, color: 'text.secondary', mb: 2 }} />
      
      <Typography variant="h5" gutterBottom>
        No Balance Sheet Data Available
      </Typography>
      
      <Typography variant="body1" color="text.secondary" paragraph>
        Your balance sheet is empty. This could be because:
      </Typography>

      <Grid container spacing={3} sx={{ mb: 4, mt: 2 }}>
        <Grid size={{ xs: 12, md: 4 }}>
          <Card variant="outlined" sx={{ height: '100%' }}>
            <CardContent>
              <AddIcon sx={{ fontSize: 40, color: 'primary.main', mb: 1 }} />
              <Typography variant="h6" gutterBottom>
                No Accounts
              </Typography>
              <Typography variant="body2" color="text.secondary">
                You need to create asset, liability, and equity accounts in your chart of accounts.
              </Typography>
            </CardContent>
          </Card>
        </Grid>
        
        <Grid size={{ xs: 12, md: 4 }}>
          <Card variant="outlined" sx={{ height: '100%' }}>
            <CardContent>
              <ReceiptIcon sx={{ fontSize: 40, color: 'primary.main', mb: 1 }} />
              <Typography variant="h6" gutterBottom>
                No Journal Entries
              </Typography>
              <Typography variant="body2" color="text.secondary">
                You need to post journal entries that affect balance sheet accounts.
              </Typography>
            </CardContent>
          </Card>
        </Grid>
        
        <Grid size={{ xs: 12, md: 4 }}>
          <Card variant="outlined" sx={{ height: '100%' }}>
            <CardContent>
              <DataArrayIcon sx={{ fontSize: 40, color: 'primary.main', mb: 1 }} />
              <Typography variant="h6" gutterBottom>
                New System
              </Typography>
              <Typography variant="body2" color="text.secondary">
                If you've just set up the system, you may need to import your opening balances.
              </Typography>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {result && (
        <Alert 
          severity={result.success ? "success" : "error"} 
          sx={{ mb: 3, textAlign: 'left', whiteSpace: 'pre-line' }}
        >
          {result.message}
        </Alert>
      )}

      <Box sx={{ mt: 3 }}>
        <Button
          variant="contained"
          color="primary"
          size="large"
          startIcon={isGenerating ? <CircularProgress size={20} color="inherit" /> : <DataArrayIcon />}
          onClick={handleGenerateSampleData}
          disabled={isGenerating}
          sx={{ mr: 2 }}
        >
          {isGenerating ? 'Generating...' : 'Generate Sample Data'}
        </Button>
      </Box>
      
      <Typography variant="body2" color="text.secondary" sx={{ mt: 3 }}>
        The sample data generator will create a complete chart of accounts with balances
        for testing the balance sheet functionality.
      </Typography>
    </Box>
  );
};

export default BalanceSheetEmptyState;
