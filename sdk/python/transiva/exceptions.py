# SPDX-License-Identifier: Apache-2.0

"""HyperSDK exceptions."""


class HyperSDKError(Exception):
    """Base exception for all HyperSDK errors."""
    pass


class AuthenticationError(HyperSDKError):
    """Authentication failed."""
    pass


class JobNotFoundError(HyperSDKError):
    """Job not found."""
    pass


class APIError(HyperSDKError):
    """API request failed."""

    def __init__(self, message: str, status_code: int = None, response: dict = None):
        super().__init__(message)
        self.status_code = status_code
        self.response = response


class ValidationError(HyperSDKError):
    """Request validation failed."""
    pass


class WebSocketError(HyperSDKError):
    """WebSocket connection error."""
    pass
