import React, { useState, useEffect } from 'react';
import {
  Box,
  Container,
  Typography,
  Card,
  CardContent,
  Button,
  Alert,
  CircularProgress,
  Grid,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Chip,
  List,
  ListItem,
  ListItemText,
  ListItemIcon,
} from '@mui/material';
import {
  DataArray as DataArrayIcon,
  Delete as DeleteIcon,
  Refresh as RefreshIcon,
  AccountBalance as AccountBalanceIcon,
  TrendingUp as TrendingUpIcon,
  Receipt as ReceiptIcon,
  Warning as WarningIcon,
} from '@mui/icons-material';
import { sampleDataService } from '../services/sampleDataService';
import PageHeader from '../components/shared/PageHeader';

interface SampleDataState {
  exists: boolean;
  loading: boolean;
}

const SampleDataManagementPage: React.FC = () => {
  const [sampleDataState, setSampleDataState] = useState<SampleDataState>({
    exists: false,
    loading: true,
  });
  const [isGenerating, setIsGenerating] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [result, setResult] = useState<{
    success?: boolean;
    message?: string;
  } | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  useEffect(() => {
    checkSampleDataStatus();
  }, []);

  const checkSampleDataStatus = async () => {
    setSampleDataState(prev => ({ ...prev, loading: true }));
    try {
      const exists = await sampleDataService.checkSampleDataExists();
      setSampleDataState({ exists, loading: false });
    } catch (error) {
      console.error('Error checking sample data status:', error);
      setSampleDataState({ exists: false, loading: false });
    }
  };

  const handleGenerateSampleData = async () => {
    setIsGenerating(true);
    setResult(null);
    
    try {
      const response = await sampleDataService.createSampleBalanceSheetData();
      setResult(response);
      
      if (response.success) {
        // Refresh status after generation
        setTimeout(() => {
          checkSampleDataStatus();
        }, 1000);
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

  const handleDeleteSampleData = async () => {
    setIsDeleting(true);
    setResult(null);
    
    try {
      const response = await sampleDataService.deleteSampleBalanceSheetData();
      setResult(response);
      setDeleteDialogOpen(false);
      
      if (response.success) {
        // Refresh status after deletion
        setTimeout(() => {
          checkSampleDataStatus();
        }, 1000);
      }
    } catch (error) {
      setResult({
        success: false,
        message: `Error deleting sample data: ${error instanceof Error ? error.message : 'Unknown error'}`
      });
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <Container maxWidth="lg" sx={{ mt: 4, mb: 4 }}>
      <PageHeader 
        title="Sample Data Management" 
        subtitle="Manage test data for financial reports"
        icon={<DataArrayIcon />}
      />

      <Grid container spacing={3}>
        {/* Status Card */}
        <Grid size={{ xs: 12, md: 6 }}>
          <Card>
            <CardContent>
              <Box display="flex" alignItems="center" mb={2}>
                <AccountBalanceIcon sx={{ mr: 1, color: 'primary.main' }} />
                <Typography variant="h6">Balance Sheet Sample Data</Typography>
              </Box>
              
              {sampleDataState.loading ? (
                <Box display="flex" alignItems="center" gap={1}>
                  <CircularProgress size={20} />
                  <Typography color="text.secondary">Checking status...</Typography>
                </Box>
              ) : (
                <Box display="flex" alignItems="center" gap={1} mb={2}>
                  <Chip
                    label={sampleDataState.exists ? "Exists" : "Not Found"}
                    color={sampleDataState.exists ? "success" : "warning"}
                    size="small"
                  />
                  <Button
                    size="small"
                    variant="outlined"
                    startIcon={<RefreshIcon />}
                    onClick={checkSampleDataStatus}
                  >
                    Check Status
                  </Button>
                </Box>
              )}

              <Typography variant="body2" color="text.secondary" paragraph>
                {sampleDataState.exists 
                  ? "Sample balance sheet accounts with balances are available for testing."
                  : "No sample data found. Generate sample accounts to test balance sheet functionality."
                }
              </Typography>

              <Box mt={2} display="flex" gap={1} flexWrap="wrap">
                <Button
                  variant="contained"
                  color="primary"
                  startIcon={isGenerating ? <CircularProgress size={20} color="inherit" /> : <DataArrayIcon />}
                  onClick={handleGenerateSampleData}
                  disabled={isGenerating || isDeleting}
                >
                  {isGenerating ? 'Generating...' : 'Generate Sample Data'}
                </Button>
                
                {sampleDataState.exists && (
                  <Button
                    variant="outlined"
                    color="error"
                    startIcon={<DeleteIcon />}
                    onClick={() => setDeleteDialogOpen(true)}
                    disabled={isGenerating || isDeleting}
                  >
                    Delete Sample Data
                  </Button>
                )}
              </Box>
            </CardContent>
          </Card>
        </Grid>

        {/* Sample Data Details Card */}
        <Grid size={{ xs: 12, md: 6 }}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                What Gets Generated
              </Typography>
              
              <List dense>
                <ListItem>
                  <ListItemIcon>
                    <TrendingUpIcon color="success" />
                  </ListItemIcon>
                  <ListItemText
                    primary="Asset Accounts"
                    secondary="Cash, Bank, Receivables, Inventory, Equipment, Vehicles, Buildings"
                  />
                </ListItem>
                
                <ListItem>
                  <ListItemIcon>
                    <ReceiptIcon color="warning" />
                  </ListItemIcon>
                  <ListItemText
                    primary="Liability Accounts"
                    secondary="Payables, Tax Payable, Accrued Expenses, Bank Loans"
                  />
                </ListItem>
                
                <ListItem>
                  <ListItemIcon>
                    <AccountBalanceIcon color="primary" />
                  </ListItemIcon>
                  <ListItemText
                    primary="Equity Accounts"
                    secondary="Share Capital, Retained Earnings"
                  />
                </ListItem>
              </List>

              <Alert severity="info" sx={{ mt: 2 }}>
                <Typography variant="body2">
                  <strong>Total Test Data:</strong><br />
                  • Assets: IDR 575,000,000<br />
                  • Liabilities: IDR 183,000,000<br />
                  • Equity: IDR 475,000,000<br />
                  <em>Books are balanced for accurate testing</em>
                </Typography>
              </Alert>
            </CardContent>
          </Card>
        </Grid>

        {/* Result Alert */}
        {result && (
          <Grid size={{ xs: 12 }}>
            <Alert 
              severity={result.success ? "success" : "error"}
              sx={{ whiteSpace: 'pre-line' }}
            >
              {result.message}
            </Alert>
          </Grid>
        )}

        {/* Usage Instructions */}
        <Grid size={{ xs: 12 }}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                How to Use Sample Data
              </Typography>
              
              <Typography variant="body2" color="text.secondary" paragraph>
                1. <strong>Generate Sample Data:</strong> Click the "Generate Sample Data" button to create a complete chart of accounts with realistic balances.
              </Typography>
              
              <Typography variant="body2" color="text.secondary" paragraph>
                2. <strong>Test Balance Sheet:</strong> Navigate to the Balance Sheet report to see the generated data in action.
              </Typography>
              
              <Typography variant="body2" color="text.secondary" paragraph>
                3. <strong>Journal Integration:</strong> Use the Enhanced P&L Report to drill down into journal entries and see how the data flows through the system.
              </Typography>
              
              <Typography variant="body2" color="text.secondary" paragraph>
                4. <strong>Clean Up:</strong> When done testing, use the "Delete Sample Data" button to remove all generated accounts.
              </Typography>

              <Alert severity="warning" sx={{ mt: 2 }}>
                <Typography variant="body2">
                  <strong>⚠️ Important:</strong> This is for testing purposes only. 
                  Do not use this feature in a production environment with real financial data.
                </Typography>
              </Alert>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {/* Delete Confirmation Dialog */}
      <Dialog
        open={deleteDialogOpen}
        onClose={() => setDeleteDialogOpen(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>
          <Box display="flex" alignItems="center" gap={1}>
            <WarningIcon color="warning" />
            Confirm Sample Data Deletion
          </Box>
        </DialogTitle>
        <DialogContent>
          <Typography>
            Are you sure you want to delete all sample balance sheet data? 
            This action will remove all generated accounts and cannot be undone.
          </Typography>
          <Alert severity="warning" sx={{ mt: 2 }}>
            This will delete approximately 20+ sample accounts from your chart of accounts.
          </Alert>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteDialogOpen(false)}>
            Cancel
          </Button>
          <Button
            onClick={handleDeleteSampleData}
            color="error"
            variant="contained"
            startIcon={isDeleting ? <CircularProgress size={20} color="inherit" /> : <DeleteIcon />}
            disabled={isDeleting}
          >
            {isDeleting ? 'Deleting...' : 'Delete Sample Data'}
          </Button>
        </DialogActions>
      </Dialog>
    </Container>
  );
};

export default SampleDataManagementPage;
