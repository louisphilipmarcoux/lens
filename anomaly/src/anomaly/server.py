"""gRPC server for the anomaly detection service."""

import json
import logging
from concurrent import futures

import grpc

from .detector import SeasonalDetector, MultiMetricCorrelator

logger = logging.getLogger(__name__)


class AnomalyService:
    """Anomaly detection gRPC service.

    Accepts metric data points and returns anomaly events.
    Can also be used as an HTTP service via a simple REST wrapper.
    """

    def __init__(
        self,
        warning_sigma: float = 2.5,
        critical_sigma: float = 3.5,
        window_size: int = 1440,
    ):
        self.detector = SeasonalDetector(
            window_size=window_size,
            warning_sigma=warning_sigma,
            critical_sigma=critical_sigma,
        )
        self.correlator = MultiMetricCorrelator()
        self.anomaly_count = 0
        self.total_points = 0

    def ingest_point(self, metric_name: str, value: float, tags: dict | None = None) -> dict | None:
        """Process a single data point. Returns anomaly event dict if detected."""
        self.total_points += 1

        event = self.detector.ingest(metric_name, value, tags)
        if event is None:
            return None

        self.anomaly_count += 1
        correlated = self.correlator.add_event(event)

        result = event.to_dict()
        if correlated:
            result["correlated_metrics"] = [e.metric_name for e in correlated]

        logger.warning(
            "Anomaly detected: %s = %.2f (expected %.2f, %.1f sigma)",
            metric_name, value, event.expected, event.deviation,
        )

        return result

    def stats(self) -> dict:
        return {
            "total_points": self.total_points,
            "anomalies_detected": self.anomaly_count,
            "tracked_metrics": len(self.detector.history),
        }


def serve(port: int = 50051) -> None:
    """Start the anomaly detection service."""
    logging.basicConfig(level=logging.INFO)

    service = AnomalyService()
    logger.info("Anomaly detection service started on port %d", port)

    # For now, this is a placeholder for the gRPC server.
    # In production, this would register a protobuf service.
    # The service can also be used as a library from the Go backend
    # via subprocess communication.
    import time
    try:
        while True:
            time.sleep(3600)
    except KeyboardInterrupt:
        logger.info("Shutting down")


if __name__ == "__main__":
    serve()
