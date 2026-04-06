"""Anomaly detection service using seasonal decomposition and adaptive thresholds."""

import json
import logging
from dataclasses import dataclass, field
from datetime import datetime
from typing import Optional

import numpy as np

logger = logging.getLogger(__name__)


@dataclass
class AnomalyEvent:
    """Represents a detected anomaly."""
    metric_name: str
    timestamp: str
    value: float
    expected: float
    deviation: float
    severity: str  # "warning" or "critical"
    tags: dict = field(default_factory=dict)

    def to_dict(self) -> dict:
        return {
            "metric_name": self.metric_name,
            "timestamp": self.timestamp,
            "value": self.value,
            "expected": self.expected,
            "deviation": self.deviation,
            "severity": self.severity,
            "tags": self.tags,
        }


class SeasonalDetector:
    """Detects anomalies using seasonal decomposition with adaptive thresholds.

    Uses a rolling window of historical data to compute expected values
    and standard deviation. Points that deviate beyond the configured
    threshold are flagged as anomalies.
    """

    def __init__(
        self,
        window_size: int = 1440,  # 24h at 1-min intervals
        warning_sigma: float = 2.5,
        critical_sigma: float = 3.5,
        min_samples: int = 60,
        seasonal_period: Optional[int] = None,
    ):
        self.window_size = window_size
        self.warning_sigma = warning_sigma
        self.critical_sigma = critical_sigma
        self.min_samples = min_samples
        self.seasonal_period = seasonal_period or window_size
        self.history: dict[str, list[float]] = {}

    def ingest(self, metric_name: str, value: float, tags: Optional[dict] = None) -> Optional[AnomalyEvent]:
        """Ingest a new data point and check for anomalies.

        Returns an AnomalyEvent if the point is anomalous, None otherwise.
        """
        key = self._key(metric_name, tags)

        if key not in self.history:
            self.history[key] = []

        self.history[key].append(value)

        # Trim to window size.
        if len(self.history[key]) > self.window_size:
            self.history[key] = self.history[key][-self.window_size:]

        # Need minimum samples before detecting.
        if len(self.history[key]) < self.min_samples:
            return None

        data = np.array(self.history[key][:-1])  # exclude current point
        expected = self._expected_value(data)
        std = self._adaptive_std(data)

        if std == 0:
            return None

        deviation = abs(value - expected) / std

        severity = None
        if deviation >= self.critical_sigma:
            severity = "critical"
        elif deviation >= self.warning_sigma:
            severity = "warning"

        if severity is None:
            return None

        return AnomalyEvent(
            metric_name=metric_name,
            timestamp=datetime.utcnow().isoformat() + "Z",
            value=value,
            expected=round(expected, 4),
            deviation=round(deviation, 2),
            severity=severity,
            tags=tags or {},
        )

    def _expected_value(self, data: np.ndarray) -> float:
        """Compute expected value using seasonal decomposition if enough data,
        otherwise fall back to exponential moving average."""
        if len(data) >= self.seasonal_period * 2:
            # Use seasonal component: average of values at same phase.
            period = self.seasonal_period
            n_periods = len(data) // period
            tail = data[-(n_periods * period):]
            reshaped = tail.reshape(n_periods, period)
            seasonal_avg = reshaped.mean(axis=0)
            return float(seasonal_avg[-1])

        # Fallback: exponential moving average.
        alpha = 2 / (min(len(data), 30) + 1)
        ema = data[0]
        for v in data[1:]:
            ema = alpha * v + (1 - alpha) * ema
        return float(ema)

    def _adaptive_std(self, data: np.ndarray) -> float:
        """Compute adaptive standard deviation using recent data."""
        recent = data[-min(len(data), 120):]  # last 2 hours
        return float(np.std(recent))

    def _key(self, metric_name: str, tags: Optional[dict]) -> str:
        if tags:
            tag_str = ",".join(f"{k}={v}" for k, v in sorted(tags.items()))
            return f"{metric_name}|{tag_str}"
        return metric_name


class MultiMetricCorrelator:
    """Detects correlated anomalies across multiple metrics."""

    def __init__(self, window: int = 5):
        self.window = window  # number of recent events to consider
        self.recent_events: list[AnomalyEvent] = []

    def add_event(self, event: AnomalyEvent) -> list[AnomalyEvent]:
        """Add an anomaly event and return correlated events if any."""
        self.recent_events.append(event)
        if len(self.recent_events) > 100:
            self.recent_events = self.recent_events[-100:]

        # Find events within the correlation window that share tags.
        correlated = []
        for prev in self.recent_events[-self.window - 1:-1]:
            if prev.metric_name != event.metric_name:
                # Check for shared tags (same host, service, etc.)
                shared = set(prev.tags.items()) & set(event.tags.items())
                if shared:
                    correlated.append(prev)

        return correlated
