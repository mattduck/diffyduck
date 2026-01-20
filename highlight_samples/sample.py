#!/usr/bin/env python3
"""Sample Python file for testing syntax highlighting."""

import os
from typing import Optional, List
from dataclasses import dataclass

# Constants
MAX_RETRIES = 3
DEFAULT_TIMEOUT = 30.5
ENABLED = True

@dataclass
class Config:
    """Configuration settings."""
    name: str
    debug: bool = False
    timeout: float = DEFAULT_TIMEOUT
    tags: Optional[List[str]] = None


def fetch_data(url: str, retries: int = MAX_RETRIES) -> Optional[dict]:
    """Fetch data from a URL with retries."""
    for attempt in range(retries):
        try:
            # Simulated fetch
            if url.startswith("https://"):
                return {"status": "ok", "attempt": attempt + 1}
            else:
                raise ValueError(f"Invalid URL: {url}")
        except Exception as e:
            print(f"Attempt {attempt + 1} failed: {e}")
            if attempt == retries - 1:
                return None
    return None


class DataProcessor:
    """Processes data from various sources."""

    def __init__(self, config: Config):
        self.config = config
        self._cache: dict = {}

    def process(self, items: List[str]) -> List[str]:
        """Process a list of items."""
        results = []
        for item in items:
            if item not in self._cache:
                self._cache[item] = item.upper()
            results.append(self._cache[item])
        return results

    @staticmethod
    def validate(data: dict) -> bool:
        """Validate data structure."""
        return "status" in data and data["status"] == "ok"


# Lambda and comprehensions
square = lambda x: x ** 2
squares = [square(i) for i in range(10)]
even_squares = {x: square(x) for x in range(10) if x % 2 == 0}

# Main entry point
if __name__ == "__main__":
    config = Config(name="test", debug=True, tags=["a", "b"])
    processor = DataProcessor(config)

    data = fetch_data("https://example.com/api")
    if data is not None and DataProcessor.validate(data):
        print(f"Success: {data}")
    else:
        print("Failed to fetch data")
