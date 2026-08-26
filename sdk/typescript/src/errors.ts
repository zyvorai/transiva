// SPDX-License-Identifier: Apache-2.0

/**
 * HyperSDK error classes
 */

export class HyperSDKError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'HyperSDKError';
  }
}

export class AuthenticationError extends HyperSDKError {
  constructor(message: string) {
    super(message);
    this.name = 'AuthenticationError';
  }
}

export class JobNotFoundError extends HyperSDKError {
  constructor(message: string) {
    super(message);
    this.name = 'JobNotFoundError';
  }
}

export class APIError extends HyperSDKError {
  public statusCode?: number;
  public response?: any;

  constructor(message: string, statusCode?: number, response?: any) {
    super(message);
    this.name = 'APIError';
    this.statusCode = statusCode;
    this.response = response;
  }
}

export class ValidationError extends HyperSDKError {
  constructor(message: string) {
    super(message);
    this.name = 'ValidationError';
  }
}

export class WebSocketError extends HyperSDKError {
  constructor(message: string) {
    super(message);
    this.name = 'WebSocketError';
  }
}
