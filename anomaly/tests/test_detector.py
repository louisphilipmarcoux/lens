"""Tests for the anomaly detector."""

import pytest
from anomaly.detector import SeasonalDetector, AnomalyEvent, MultiMetricCorrelator


class TestSeasonalDetector:
    def test_no_anomaly_on_normal_data(self):
        detector = SeasonalDetector(window_size=100, min_samples=10)

        # Feed stable data.
        for i in range(50):
            result = detector.ingest("cpu", 50.0 + (i % 3))

        # Normal value should not be anomalous.
        result = detector.ingest("cpu", 51.0)
        assert result is None

    def test_detects_spike(self):
        detector = SeasonalDetector(
            window_size=100,
            min_samples=10,
            warning_sigma=2.0,
            critical_sigma=3.0,
        )

        # Feed stable baseline.
        for _ in range(60):
            detector.ingest("cpu", 50.0)

        # Inject a massive spike.
        result = detector.ingest("cpu", 500.0)
        assert result is not None
        assert result.severity in ("warning", "critical")
        assert result.value == 500.0
        assert result.metric_name == "cpu"

    def test_requires_minimum_samples(self):
        detector = SeasonalDetector(min_samples=20)

        for i in range(15):
            result = detector.ingest("metric", 100.0)
            assert result is None  # not enough samples yet

    def test_separate_metrics(self):
        detector = SeasonalDetector(window_size=100, min_samples=10)

        for _ in range(30):
            detector.ingest("cpu", 50.0)
            detector.ingest("memory", 70.0)

        # Each metric has its own history.
        assert len(detector.history) == 2

    def test_tags_create_separate_series(self):
        detector = SeasonalDetector(window_size=100, min_samples=10)

        for _ in range(30):
            detector.ingest("cpu", 50.0, {"host": "web-01"})
            detector.ingest("cpu", 80.0, {"host": "web-02"})

        assert len(detector.history) == 2

    def test_anomaly_event_to_dict(self):
        event = AnomalyEvent(
            metric_name="cpu",
            timestamp="2026-01-01T00:00:00Z",
            value=99.0,
            expected=50.0,
            deviation=3.5,
            severity="critical",
            tags={"host": "web-01"},
        )
        d = event.to_dict()
        assert d["metric_name"] == "cpu"
        assert d["severity"] == "critical"
        assert d["tags"]["host"] == "web-01"


class TestMultiMetricCorrelator:
    def test_detects_correlated_anomalies(self):
        correlator = MultiMetricCorrelator(window=5)

        e1 = AnomalyEvent("cpu", "t1", 99, 50, 3.5, "critical", {"host": "web-01"})
        corr1 = correlator.add_event(e1)
        assert len(corr1) == 0

        e2 = AnomalyEvent("memory", "t2", 95, 60, 3.0, "warning", {"host": "web-01"})
        corr2 = correlator.add_event(e2)
        assert len(corr2) == 1
        assert corr2[0].metric_name == "cpu"

    def test_no_correlation_different_hosts(self):
        correlator = MultiMetricCorrelator(window=5)

        e1 = AnomalyEvent("cpu", "t1", 99, 50, 3.5, "critical", {"host": "web-01"})
        correlator.add_event(e1)

        e2 = AnomalyEvent("memory", "t2", 95, 60, 3.0, "warning", {"host": "web-02"})
        corr = correlator.add_event(e2)
        assert len(corr) == 0
