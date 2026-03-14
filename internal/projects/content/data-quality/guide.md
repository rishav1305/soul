# Data Quality Framework: Validation, Profiling, and Anomaly Detection

## Overview

Data quality is the foundation of every ML system. Models trained on dirty data produce unreliable predictions. Pipelines that don't validate inputs fail silently, propagating errors downstream for hours before anyone notices. A data quality framework catches problems at the point of ingestion — schema violations, missing values, distribution shifts, outliers, and format inconsistencies.

This project builds a comprehensive data quality toolkit: validation rules with Great Expectations, automated profiling with ydata-profiling, statistical anomaly detection with scipy, and schema enforcement with Pandera. These skills are essential for data engineering and ML engineering roles — data quality is consistently cited as the top challenge in production ML systems.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                   Data Quality Framework                         │
│                                                                  │
│  ┌──────────┐   ┌──────────────┐   ┌──────────────┐            │
│  │ Data      │──▶│ Schema       │──▶│ Statistical  │            │
│  │ Source    │   │ Validation   │   │ Validation   │            │
│  └──────────┘   │ (Pandera)    │   │ (GX)         │            │
│                  └──────┬───────┘   └──────┬───────┘            │
│                         │                  │                     │
│                         ▼                  ▼                     │
│                  ┌──────────────┐   ┌──────────────┐            │
│                  │ Profiling    │   │ Anomaly      │            │
│                  │ Report       │   │ Detection    │            │
│                  │ (ydata)      │   │ (scipy)      │            │
│                  └──────┬───────┘   └──────┬───────┘            │
│                         │                  │                     │
│                         ▼                  ▼                     │
│                  ┌─────────────────────────────────┐            │
│                  │       Quality Report            │            │
│                  │  - Pass/fail status              │            │
│                  │  - Violation details              │            │
│                  │  - Drift metrics                  │            │
│                  │  - Recommendations                │            │
│                  └─────────────────────────────────┘            │
└──────────────────────────────────────────────────────────────────┘
```

**Components:**

- **Schema Validation (Pandera)** — Enforces column types, value ranges, and custom constraints at the DataFrame level. Acts as a contract between data producers and consumers.
- **Statistical Validation (Great Expectations)** — Tests data against statistical expectations: distribution properties, uniqueness, referential integrity, and aggregate metrics.
- **Profiling (ydata-profiling)** — Generates comprehensive statistical profiles of datasets: distributions, correlations, missing patterns, and duplicates.
- **Anomaly Detection (scipy)** — Identifies individual records or batches that deviate significantly from expected distributions.
- **Quality Report** — Aggregates all validation results into an actionable report with pass/fail status and remediation suggestions.

## Key Concepts

### Data Contracts

A data contract is a formal specification of what valid data looks like. It defines: column names and types, allowed value ranges, nullability rules, uniqueness constraints, and referential integrity. Data contracts act as an interface between teams — the data producer guarantees the contract, and the consumer relies on it. When a contract is violated, the pipeline fails fast with a clear error rather than silently producing wrong results.

### Types of Data Quality Issues

**Structural issues**: Wrong column names, incorrect types (string instead of integer), missing columns. These break code immediately.

**Content issues**: Null values, out-of-range values, invalid formats (malformed dates, wrong currency codes). These cause subtle bugs — a null in a feature column becomes NaN in the model, which might silently produce a default prediction.

**Distributional issues**: The data is structurally valid but statistically different from what's expected. Mean shifted, variance increased, a new category appeared, or the class balance changed. These indicate data drift and degrade model performance over time.

**Temporal issues**: Duplicate records, out-of-order timestamps, gaps in time series, stale data. These are common in streaming pipelines and can cause training-serving skew.

### Statistical Testing for Drift

To detect whether a new batch of data differs from the reference (training) distribution:

- **Kolmogorov-Smirnov test (KS test)**: Compares two continuous distributions. p-value < 0.05 suggests significant drift. Works well for univariate comparisons.
- **Population Stability Index (PSI)**: Measures how much a distribution has shifted. PSI < 0.1 is stable, 0.1-0.2 is moderate change, >0.2 is significant drift.
- **Chi-squared test**: For categorical variables. Tests whether the frequency distribution of categories has changed significantly.
- **Jensen-Shannon divergence**: Symmetric measure of distribution similarity. Bounded between 0 (identical) and 1 (completely different).

### Great Expectations vs Pandera

Both validate DataFrames, but they serve different purposes. **Pandera** is lightweight, Pythonic, and integrates with type checking — think of it as a schema for DataFrames, similar to Pydantic for APIs. Use it for quick, inline validation in scripts. **Great Expectations** is a full framework with data docs, profiling, checkpoints, and integrations with orchestrators (Airflow, Dagster). Use it for production pipelines where you need audit trails, shared expectations across teams, and HTML reports.

## Implementation Steps

### Step 1: Environment Setup

```python
# requirements.txt
great-expectations==0.18.21
ydata-profiling==4.9.0
pandera==0.20.4
scipy==1.14.1
pandas==2.2.2
numpy==2.1.1
```

### Step 2: Schema Validation with Pandera

```python
# schema.py
import pandera as pa
from pandera import Column, Check, DataFrameSchema, Index
import pandas as pd

# Define schema as a contract
transaction_schema = DataFrameSchema(
    columns={
        "transaction_id": Column(
            str,
            checks=[
                Check.str_matches(r"^TXN-\d{10}$"),  # Format: TXN-0000000001
            ],
            unique=True,
            nullable=False,
        ),
        "amount": Column(
            float,
            checks=[
                Check.greater_than(0),
                Check.less_than(1_000_000),  # Max transaction amount
            ],
            nullable=False,
        ),
        "currency": Column(
            str,
            checks=[
                Check.isin(["USD", "EUR", "GBP", "JPY", "CAD"]),
            ],
            nullable=False,
        ),
        "timestamp": Column(
            "datetime64[ns]",
            nullable=False,
        ),
        "customer_id": Column(
            str,
            checks=[
                Check.str_length(min_value=5, max_value=20),
            ],
            nullable=False,
        ),
        "category": Column(
            str,
            checks=[
                Check.isin([
                    "retail", "food", "travel", "entertainment",
                    "utilities", "healthcare", "other"
                ]),
            ],
            nullable=False,
        ),
        "is_fraud": Column(
            bool,
            nullable=False,
        ),
    },
    checks=[
        # DataFrame-level checks
        Check(lambda df: df["timestamp"].is_monotonic_increasing,
              error="Timestamps must be in order"),
        Check(lambda df: len(df) >= 100,
              error="Batch must have at least 100 records"),
    ],
    coerce=True,  # Attempt type coercion before validation
)

def validate_schema(df: pd.DataFrame) -> pd.DataFrame:
    """Validate DataFrame against schema. Raises SchemaError on failure."""
    return transaction_schema.validate(df, lazy=True)
```

### Step 3: Statistical Validation with Great Expectations

```python
# expectations.py
import great_expectations as gx
import pandas as pd

def create_expectation_suite(reference_df: pd.DataFrame) -> dict:
    """Create expectations from a reference (training) dataset."""
    context = gx.get_context()

    datasource = context.sources.add_or_update_pandas("pandas_source")
    asset = datasource.add_dataframe_asset("reference_data")
    batch_request = asset.build_batch_request(dataframe=reference_df)

    validator = context.get_validator(batch_request=batch_request)

    # Completeness expectations
    for col in reference_df.columns:
        null_pct = reference_df[col].isnull().mean()
        validator.expect_column_values_to_not_be_null(
            column=col, mostly=1.0 - null_pct - 0.01  # 1% tolerance
        )

    # Distribution expectations for numeric columns
    for col in reference_df.select_dtypes(include="number").columns:
        mean = reference_df[col].mean()
        std = reference_df[col].std()
        validator.expect_column_mean_to_be_between(
            column=col,
            min_value=mean - 3 * std,
            max_value=mean + 3 * std,
        )
        validator.expect_column_stdev_to_be_between(
            column=col,
            min_value=std * 0.5,
            max_value=std * 2.0,
        )

    # Categorical distribution expectations
    for col in reference_df.select_dtypes(include="object").columns:
        values = reference_df[col].unique().tolist()
        validator.expect_column_values_to_be_in_set(
            column=col, value_set=values
        )

    # Row count expectations (within 50% of reference)
    n_rows = len(reference_df)
    validator.expect_table_row_count_to_be_between(
        min_value=int(n_rows * 0.5),
        max_value=int(n_rows * 2.0),
    )

    return validator.get_expectation_suite()

def validate_batch(df: pd.DataFrame, suite) -> dict:
    """Validate a new batch against the expectation suite."""
    context = gx.get_context()
    datasource = context.sources.add_or_update_pandas("pandas_source")
    asset = datasource.add_dataframe_asset("batch_data")
    batch_request = asset.build_batch_request(dataframe=df)

    validator = context.get_validator(batch_request=batch_request)
    results = validator.validate(expectation_suite=suite)

    return {
        "success": results.success,
        "statistics": results.statistics,
        "failures": [
            {
                "expectation": r.expectation_config.expectation_type,
                "column": r.expectation_config.kwargs.get("column"),
                "details": str(r.result),
            }
            for r in results.results if not r.success
        ],
    }
```

### Step 4: Distribution Drift Detection

```python
# drift.py
import numpy as np
import pandas as pd
from scipy import stats
from typing import Optional

def compute_psi(reference: np.ndarray, current: np.ndarray,
                n_bins: int = 10) -> float:
    """Compute Population Stability Index between two distributions."""
    # Create bins from reference distribution
    bins = np.percentile(reference, np.linspace(0, 100, n_bins + 1))
    bins[0] = -np.inf
    bins[-1] = np.inf

    ref_counts = np.histogram(reference, bins=bins)[0]
    cur_counts = np.histogram(current, bins=bins)[0]

    # Avoid division by zero
    ref_pct = (ref_counts + 1) / (len(reference) + n_bins)
    cur_pct = (cur_counts + 1) / (len(current) + n_bins)

    psi = np.sum((cur_pct - ref_pct) * np.log(cur_pct / ref_pct))
    return float(psi)

def detect_drift(reference_df: pd.DataFrame, current_df: pd.DataFrame,
                 threshold_psi: float = 0.2,
                 threshold_ks_pvalue: float = 0.05) -> dict:
    """Detect distribution drift between reference and current datasets."""
    results = {}

    for col in reference_df.select_dtypes(include="number").columns:
        ref_values = reference_df[col].dropna().values
        cur_values = current_df[col].dropna().values

        # KS test
        ks_stat, ks_pvalue = stats.ks_2samp(ref_values, cur_values)

        # PSI
        psi = compute_psi(ref_values, cur_values)

        # Mean shift (in standard deviations)
        ref_mean = np.mean(ref_values)
        ref_std = np.std(ref_values) or 1.0
        mean_shift = abs(np.mean(cur_values) - ref_mean) / ref_std

        drifted = psi > threshold_psi or ks_pvalue < threshold_ks_pvalue
        results[col] = {
            "psi": round(psi, 4),
            "ks_statistic": round(ks_stat, 4),
            "ks_pvalue": round(ks_pvalue, 4),
            "mean_shift_std": round(mean_shift, 4),
            "drifted": drifted,
            "severity": "high" if psi > 0.25 else "medium" if psi > 0.1 else "low",
        }

    # Categorical drift using chi-squared test
    for col in reference_df.select_dtypes(include="object").columns:
        ref_counts = reference_df[col].value_counts(normalize=True)
        cur_counts = current_df[col].value_counts(normalize=True)

        # Align categories
        all_cats = set(ref_counts.index) | set(cur_counts.index)
        ref_freq = np.array([ref_counts.get(c, 0) for c in all_cats])
        cur_freq = np.array([cur_counts.get(c, 0) for c in all_cats])

        # New categories detected
        new_cats = set(cur_counts.index) - set(ref_counts.index)

        if len(ref_freq) > 1:
            chi2, p_value = stats.chisquare(
                cur_freq * len(current_df),
                f_exp=ref_freq * len(current_df)
            )
        else:
            chi2, p_value = 0, 1.0

        results[col] = {
            "chi2_statistic": round(chi2, 4),
            "chi2_pvalue": round(p_value, 4),
            "new_categories": list(new_cats),
            "drifted": p_value < threshold_ks_pvalue or len(new_cats) > 0,
        }

    return results
```

### Step 5: Anomaly Detection at Record Level

```python
# anomalies.py
import numpy as np
import pandas as pd
from scipy import stats

def detect_outliers(df: pd.DataFrame,
                    method: str = "iqr",
                    threshold: float = 3.0) -> pd.DataFrame:
    """Flag individual records with anomalous values.

    Methods:
      - 'iqr': Interquartile range (robust to non-normal data)
      - 'zscore': Standard deviation from mean (assumes normality)
      - 'isolation': Isolation forest (multivariate)
    """
    numeric_cols = df.select_dtypes(include="number").columns
    anomaly_flags = pd.DataFrame(index=df.index)

    for col in numeric_cols:
        values = df[col].dropna()

        if method == "iqr":
            q1 = values.quantile(0.25)
            q3 = values.quantile(0.75)
            iqr = q3 - q1
            lower = q1 - threshold * iqr
            upper = q3 + threshold * iqr
            anomaly_flags[f"{col}_outlier"] = (
                (df[col] < lower) | (df[col] > upper)
            )

        elif method == "zscore":
            z_scores = np.abs(stats.zscore(values))
            anomaly_flags[f"{col}_outlier"] = False
            anomaly_flags.loc[values.index, f"{col}_outlier"] = z_scores > threshold

    # Overall anomaly score: number of flagged columns per record
    outlier_cols = [c for c in anomaly_flags.columns if c.endswith("_outlier")]
    anomaly_flags["anomaly_score"] = anomaly_flags[outlier_cols].sum(axis=1)
    anomaly_flags["is_anomaly"] = anomaly_flags["anomaly_score"] > 0

    return anomaly_flags
```

### Step 6: Automated Profiling

```python
# profiling.py
from ydata_profiling import ProfileReport
import pandas as pd

def generate_profile(df: pd.DataFrame, title: str = "Data Quality Report",
                     output_path: str = "report.html",
                     minimal: bool = False) -> ProfileReport:
    """Generate a comprehensive data profiling report."""
    profile = ProfileReport(
        df,
        title=title,
        minimal=minimal,  # True for large datasets (>100k rows)
        explorative=True,
        correlations={
            "auto": {"calculate": True},
            "pearson": {"calculate": True},
            "spearman": {"calculate": True},
        },
        missing_diagrams={
            "bar": True,
            "matrix": True,
            "heatmap": True,
        },
    )
    profile.to_file(output_path)
    return profile

def compare_profiles(reference_df: pd.DataFrame,
                     current_df: pd.DataFrame,
                     output_path: str = "comparison.html"):
    """Generate a comparison report between two datasets."""
    ref_profile = ProfileReport(reference_df, title="Reference")
    cur_profile = ProfileReport(current_df, title="Current")
    comparison = ref_profile.compare(cur_profile)
    comparison.to_file(output_path)
```

## Testing & Evaluation

### Testing the Framework

- **Known-bad data**: Create test DataFrames with intentional violations (nulls, wrong types, outliers) and verify each is caught.
- **Sensitivity testing**: Gradually shift a distribution and verify the drift detector triggers at the expected threshold.
- **Performance**: Profile validation time on datasets of increasing size. Schema validation should be <1 second for 1M rows; statistical validation <10 seconds.
- **False positive rate**: Run the validator on multiple valid batches sampled from the same distribution. False positive rate should be <5%.

### Quality Metrics

- **Detection rate**: Percentage of known data issues caught by the framework.
- **False positive rate**: Percentage of valid batches flagged as problematic.
- **Time to detection**: How quickly after a data issue is introduced does the framework catch it.
- **Coverage**: Percentage of data columns with active validation rules.

## Interview Angles

### Q1: How do you decide which data quality checks to implement for a new ML pipeline?

**Sample Answer:** I start with three layers. Layer 1 (structural): schema validation — column names, types, and non-null constraints. This is non-negotiable and catches 60% of issues. I implement this with Pandera because it's lightweight and runs inline. Layer 2 (statistical): distribution checks based on the training data — means, variances, and value ranges within reasonable bounds. This catches data drift. I implement this with Great Expectations because it provides audit trails. Layer 3 (business): domain-specific rules from stakeholders — "transaction amounts should never exceed $100K" or "customer age should be 18-120." These catch upstream system bugs. The tradeoff is maintenance burden — more checks means more false positives when legitimate data changes occur. I start strict and relax checks that trigger too frequently, rather than starting loose and missing issues.

### Q2: How would you handle data quality in a real-time streaming pipeline?

**Sample Answer:** Streaming requires different strategies than batch because you can't profile the whole dataset — you see one record at a time. I use three approaches: (1) Record-level schema validation — Pandera or JSON Schema on every record. Invalid records go to a dead letter queue for investigation, not into the pipeline. (2) Windowed statistical checks — every N minutes, compute statistics on the window (mean, null rate, cardinality) and compare to reference baselines. Alert on significant deviations. (3) Watermarking — track data freshness. If no data arrives for 5 minutes when the expected frequency is 1 record/second, something is broken upstream. The tradeoff vs batch validation is granularity vs completeness — stream checks catch issues faster but can't detect subtle distributional shifts that only appear at scale. I complement stream checks with periodic batch validation on materialized data.

### Q3: What's the difference between data validation and data testing?

**Sample Answer:** Data validation runs in production on every batch of data, checking that it conforms to expectations. It's a runtime guard. Data testing runs in CI/CD on the validation logic itself, checking that the validators work correctly. For example, a data test verifies that the null check catches nulls and doesn't false-positive on valid data. A data validation runs that null check on actual production data. You need both — validation without testing means your validators might have bugs (a misconfigured regex that accepts invalid formats). Testing without validation means you've verified the logic but aren't applying it to real data. I also distinguish between "hard" validations (pipeline fails on violation) and "soft" validations (alert but continue). Hard validations are for structural issues; soft validations are for statistical drift where you want human judgment before blocking the pipeline.

### Q4: How do you manage data quality expectations as data evolves over time?

**Sample Answer:** Data distributions legitimately change — seasonal effects, new product categories, business growth. Static expectations become stale and generate false positives. My approach: (1) Rolling baselines — instead of comparing to a fixed reference, compare to the last 30 days. This naturally adapts to gradual shifts. (2) Seasonal adjustments — for known patterns (holiday traffic spikes, weekend drops), maintain per-period baselines. (3) Expected change annotations — when a business change is known (new product launch), temporarily widen thresholds or update the reference. (4) Automated baseline updates — if a check triggers but the data engineer confirms it's valid, automatically update the baseline. The key principle is that expectations should reflect what's "normal," and normal changes. The tradeoff is that auto-updating baselines can mask real problems if drift is gradual — I complement rolling baselines with absolute bounds that never auto-update (e.g., age must be positive).
