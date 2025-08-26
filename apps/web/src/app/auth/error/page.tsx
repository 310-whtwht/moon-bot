'use client';

import { Suspense } from 'react';
import { useSearchParams } from 'next/navigation';
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { AlertTriangle, ArrowLeft } from 'lucide-react';

function AuthErrorContent() {
  const searchParams = useSearchParams();
  const error = searchParams.get('error');

  const getErrorDetails = (errorCode: string | null) => {
    switch (errorCode) {
      case 'CredentialsSignin':
        return {
          title: 'Invalid Credentials',
          description: 'The email or password you entered is incorrect.',
          suggestion: 'Please check your credentials and try again.',
        };
      case 'AccessDenied':
        return {
          title: 'Access Denied',
          description: 'You do not have permission to access this resource.',
          suggestion: 'Please contact your administrator for access.',
        };
      case 'Configuration':
        return {
          title: 'Configuration Error',
          description:
            'There is a problem with the authentication configuration.',
          suggestion: 'Please contact support for assistance.',
        };
      case 'Verification':
        return {
          title: 'Verification Failed',
          description: 'The verification process could not be completed.',
          suggestion: 'Please try again or contact support.',
        };
      default:
        return {
          title: 'Authentication Error',
          description: 'An unexpected error occurred during authentication.',
          suggestion:
            'Please try again or contact support if the problem persists.',
        };
    }
  };

  const errorDetails = getErrorDetails(error);

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md w-full space-y-8">
        <div className="text-center">
          <AlertTriangle className="mx-auto h-12 w-12 text-red-600" />
          <h2 className="mt-6 text-3xl font-extrabold text-gray-900">
            Authentication Error
          </h2>
          <p className="mt-2 text-sm text-gray-600">
            We encountered an issue with your sign-in attempt
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="text-red-600">{errorDetails.title}</CardTitle>
            <CardDescription>{errorDetails.description}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <Alert>
              <AlertTriangle className="h-4 w-4" />
              <AlertDescription>{errorDetails.suggestion}</AlertDescription>
            </Alert>

            <div className="space-y-2">
              <Link href="/auth/signin">
                <Button className="w-full">
                  <ArrowLeft className="w-4 h-4 mr-2" />
                  Try Again
                </Button>
              </Link>

              <Link href="/">
                <Button variant="outline" className="w-full">
                  Return to Home
                </Button>
              </Link>
            </div>

            {error && (
              <div className="mt-4 p-3 bg-gray-100 rounded-md">
                <p className="text-xs text-gray-600">
                  <strong>Error Code:</strong> {error}
                </p>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

export default function AuthErrorPage() {
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <AuthErrorContent />
    </Suspense>
  );
}
