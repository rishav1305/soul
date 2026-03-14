# MLOps Pipeline: End-to-End ML Lifecycle Management

## Overview

MLOps bridges the gap between experimental ML notebooks and production systems. Most ML projects fail not because of poor models but because of poor engineering: unreproducible experiments, manual deployment, no monitoring, silent data drift. An MLOps pipeline automates the full lifecycle — data validation, experiment tracking, model registry, testing, deployment, and monitoring.

This project builds a production-grade pipeline using industry-standard tools. You'll learn experiment management with MLflow, data versioning with DVC, configuration management with Hydra, and data validation with Great Expectations. These skills are the core of ML engineering roles and increasingly expected in senior data science positions.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                       MLOps Pipeline                                │
│                                                                     │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────────┐    │
│  │ Data      │──▶│ Data     │──▶│ Feature  │──▶│ Training     │    │
│  │ Ingestion │   │ Validate │   │ Engineer │   │ + Experiment │    │
│  │ (DVC)     │   │ (GX)     │   │ (Hydra)  │   │ (MLflow)     │    │
│  └──────────┘   └──────────┘   └──────────┘   └──────┬───────┘    │
│                                                        │            │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐          │            │
│  │ Monitor   │◀──│ Deploy   │◀──│ Registry │◀─────────┘            │
│  │ (Drift)   │   │ (CI/CD)  │   │ (MLflow) │                      │
│  └──────────┘   └──────────┘   └──────────┘                       │
└─────────────────────────────────────────────────────────────────────┘
```

**Components:**

- **Data Ingestion (DVC)** — Tracks data versions alongside code. Raw data, processed data, and model artifacts are versioned without storing large files in Git.
- **Data Validation (Great Expectations)** — Validates data quality before training. Catches schema changes, missing values, distribution shifts, and anomalies.
- **Feature Engineering (Hydra)** — Configuration-driven feature pipelines. Every experiment's hyperparameters, data paths, and feature configs are captured in YAML.
- **Training + Experiment Tracking (MLflow)** — Logs metrics, parameters, artifacts, and code versions for every experiment run. Enables comparison and reproducibility.
- **Model Registry (MLflow)** — Promotes models through stages: None -> Staging -> Production. Each model version has lineage back to the exact data and code.
- **Deploy (CI/CD)** — Automated testing and deployment triggered by model promotion in the registry.
- **Monitor** — Tracks prediction drift, data drift, and model performance in production.

## Key Concepts

### Experiment Reproducibility

Every experiment must be fully reproducible from its record. This requires tracking five things: (1) Code version (git commit), (2) Data version (DVC hash), (3) Configuration (all hyperparameters), (4) Environment (Python version, package versions), (5) Random seeds. If any of these are missing, you cannot reproduce the result, which makes debugging regressions and comparing experiments unreliable.

### Data Versioning with DVC

DVC (Data Version Control) extends Git to handle large files. It stores file hashes in Git while the actual data lives in remote storage (S3, GCS, local). When you `dvc push`, data goes to remote storage. When you `git checkout` a branch, `dvc checkout` restores the corresponding data version. This gives you full data lineage — you can see exactly which data version produced each model.

DVC pipelines (`dvc.yaml`) define the DAG of stages: data processing -> feature engineering -> training -> evaluation. Running `dvc repro` only re-executes stages whose inputs changed, saving time. The pipeline definition is version-controlled, so the processing logic and data evolve together.

### Configuration Management with Hydra

Hydra eliminates hardcoded hyperparameters. Instead of editing code to change learning rate, you define configs in YAML and override from the command line: `python train.py lr=0.001 model=transformer data=v2`. Hydra composes configurations hierarchically — base config + model-specific config + data-specific config — and logs the final resolved config to MLflow automatically.

This is critical for systematic experimentation. You can run sweeps (`--multirun lr=0.001,0.0001,0.00001`) and every combination is tracked with its full config, making it trivial to find the best hyperparameters.

### Model Registry Workflow

The model registry enforces a promotion lifecycle:

1. **None**: Every trained model is logged to MLflow. Most stay here.
2. **Staging**: Models promoted for validation testing. Automated tests run against staging models.
3. **Production**: Models that pass validation are promoted. Only one production model per task.

Promotion can be manual (human approval) or automated (if staging model beats production model on the evaluation suite by a significant margin). Demotion happens when monitoring detects degradation.

## Implementation Steps

### Step 1: Project Structure

```
mlops-project/
├── configs/
│   ├── config.yaml          # Base config
│   ├── model/
│   │   ├── xgboost.yaml
│   │   └── lightgbm.yaml
│   └── data/
│       ├── v1.yaml
│       └── v2.yaml
├── data/
│   ├── raw/                  # DVC-tracked
│   └── processed/            # DVC-tracked
├── src/
│   ├── data_validation.py
│   ├── features.py
│   ├── train.py
│   └── evaluate.py
├── tests/
│   ├── test_data_quality.py
│   └── test_model_quality.py
├── dvc.yaml
├── dvc.lock
└── requirements.txt
```

### Step 2: Data Validation with Great Expectations

```python
# src/data_validation.py
import great_expectations as gx
import pandas as pd

def validate_training_data(df: pd.DataFrame) -> bool:
    """Validate training data quality before model training."""
    context = gx.get_context()

    # Define expectations programmatically
    validator = context.sources.pandas_default.read_dataframe(df)

    # Schema expectations
    validator.expect_table_columns_to_match_ordered_list(
        column_list=["feature_1", "feature_2", "feature_3", "target"]
    )

    # Completeness expectations
    for col in ["feature_1", "feature_2", "feature_3"]:
        validator.expect_column_values_to_not_be_null(
            column=col, mostly=0.99  # Allow 1% nulls
        )

    # Distribution expectations
    validator.expect_column_values_to_be_between(
        column="feature_1", min_value=-10, max_value=10
    )
    validator.expect_column_mean_to_be_between(
        column="feature_1", min_value=-1, max_value=1
    )
    validator.expect_column_values_to_be_in_set(
        column="target", value_set=[0, 1]
    )

    # Row count expectations
    validator.expect_table_row_count_to_be_between(
        min_value=1000, max_value=10_000_000
    )

    results = validator.validate()
    if not results.success:
        failed = [
            r.expectation_config.expectation_type
            for r in results.results if not r.success
        ]
        raise ValueError(f"Data validation failed: {failed}")
    return True
```

### Step 3: Hydra Configuration

```yaml
# configs/config.yaml
defaults:
  - model: xgboost
  - data: v1

seed: 42
test_size: 0.2

training:
  n_splits: 5  # cross-validation folds
  early_stopping_rounds: 50

mlflow:
  tracking_uri: "http://localhost:5000"
  experiment_name: "my-ml-project"
```

```yaml
# configs/model/xgboost.yaml
name: xgboost
params:
  n_estimators: 1000
  max_depth: 6
  learning_rate: 0.1
  subsample: 0.8
  colsample_bytree: 0.8
  min_child_weight: 1
  reg_alpha: 0.0
  reg_lambda: 1.0
```

### Step 4: Training with MLflow Tracking

```python
# src/train.py
import mlflow
import hydra
from omegaconf import DictConfig, OmegaConf
import xgboost as xgb
from sklearn.model_selection import cross_val_score
import pandas as pd
import numpy as np
import subprocess

def get_git_commit() -> str:
    result = subprocess.run(
        ["git", "rev-parse", "HEAD"], capture_output=True, text=True
    )
    return result.stdout.strip()

def get_dvc_hash(data_path: str) -> str:
    result = subprocess.run(
        ["dvc", "status", data_path], capture_output=True, text=True
    )
    return result.stdout.strip()

@hydra.main(config_path="../configs", config_name="config", version_base=None)
def train(cfg: DictConfig):
    mlflow.set_tracking_uri(cfg.mlflow.tracking_uri)
    mlflow.set_experiment(cfg.mlflow.experiment_name)

    with mlflow.start_run():
        # Log configuration
        mlflow.log_params(OmegaConf.to_container(cfg, resolve=True))
        mlflow.set_tag("git_commit", get_git_commit())

        # Load and validate data
        df = pd.read_parquet(f"data/processed/{cfg.data.version}/train.parquet")
        from data_validation import validate_training_data
        validate_training_data(df)

        X = df.drop("target", axis=1)
        y = df["target"]

        # Train with cross-validation
        model = xgb.XGBClassifier(**cfg.model.params, random_state=cfg.seed)
        cv_scores = cross_val_score(
            model, X, y, cv=cfg.training.n_splits, scoring="roc_auc"
        )

        mlflow.log_metric("cv_auc_mean", np.mean(cv_scores))
        mlflow.log_metric("cv_auc_std", np.std(cv_scores))

        # Train final model on full data
        model.fit(X, y)

        # Log model
        mlflow.xgboost.log_model(
            model, "model",
            registered_model_name=f"{cfg.mlflow.experiment_name}-{cfg.model.name}",
        )

        # Log feature importance
        importance = pd.DataFrame({
            "feature": X.columns,
            "importance": model.feature_importances_,
        }).sort_values("importance", ascending=False)
        mlflow.log_table(importance, "feature_importance.json")

        print(f"CV AUC: {np.mean(cv_scores):.4f} +/- {np.std(cv_scores):.4f}")

if __name__ == "__main__":
    train()
```

### Step 5: DVC Pipeline

```yaml
# dvc.yaml
stages:
  process:
    cmd: python src/features.py
    deps:
      - data/raw/
      - src/features.py
    outs:
      - data/processed/

  validate:
    cmd: python -m pytest tests/test_data_quality.py -v
    deps:
      - data/processed/
      - tests/test_data_quality.py

  train:
    cmd: python src/train.py
    deps:
      - data/processed/
      - src/train.py
      - configs/
    metrics:
      - metrics.json:
          cache: false

  evaluate:
    cmd: python src/evaluate.py
    deps:
      - src/evaluate.py
    metrics:
      - evaluation.json:
          cache: false
```

### Step 6: Model Promotion Script

```python
# src/promote_model.py
import mlflow
from mlflow.tracking import MlflowClient

def promote_if_better(model_name: str, new_run_id: str,
                      metric: str = "cv_auc_mean",
                      threshold: float = 0.01):
    """Promote a model to production if it beats the current production model."""
    client = MlflowClient()

    # Get new model's metric
    new_run = client.get_run(new_run_id)
    new_score = new_run.data.metrics[metric]

    # Get current production model's metric
    production_versions = client.get_latest_versions(
        model_name, stages=["Production"]
    )

    if not production_versions:
        # No production model yet — promote directly
        latest = client.get_latest_versions(model_name, stages=["None"])
        if latest:
            client.transition_model_version_stage(
                model_name, latest[0].version, "Production"
            )
            print(f"Promoted v{latest[0].version} to Production (first model)")
        return

    prod_version = production_versions[0]
    prod_run = client.get_run(prod_version.run_id)
    prod_score = prod_run.data.metrics[metric]

    improvement = new_score - prod_score
    print(f"Current production: {prod_score:.4f}, New model: {new_score:.4f}")
    print(f"Improvement: {improvement:.4f} (threshold: {threshold})")

    if improvement > threshold:
        # Demote current production
        client.transition_model_version_stage(
            model_name, prod_version.version, "Archived"
        )
        # Promote new model
        latest = client.get_latest_versions(model_name, stages=["None"])
        client.transition_model_version_stage(
            model_name, latest[0].version, "Production"
        )
        print(f"Promoted v{latest[0].version} to Production")
    else:
        print("New model does not beat production threshold. Not promoting.")
```

## Testing & Evaluation

### Pipeline Tests

- **Data quality tests**: Run Great Expectations suite on every data update. Fail the pipeline if expectations are violated.
- **Model quality tests**: Compare new model against baseline on a held-out test set. Assert minimum performance thresholds (e.g., AUC > 0.85).
- **Integration tests**: Run the full pipeline end-to-end on a small sample dataset to verify all stages connect correctly.
- **Reproducibility tests**: Train the same model with the same config and seed twice. Results must match exactly.

### Monitoring Metrics

- **Prediction drift**: KL divergence or PSI (Population Stability Index) between training and production prediction distributions. Alert if PSI > 0.2.
- **Feature drift**: Monitor each input feature's distribution. Flag features whose mean or variance shifts by more than 2 standard deviations.
- **Performance degradation**: If you have delayed labels, track actual performance over time. Set up alerts for drops exceeding the confidence interval.

## Interview Angles

### Q1: How do you ensure ML experiment reproducibility?

**Sample Answer:** I track five dimensions: (1) Code via git commit hash logged to MLflow, (2) Data via DVC hashes that link each experiment to exact data versions, (3) Configuration via Hydra which logs the complete resolved config, (4) Environment via pip freeze or conda lock files stored as artifacts, (5) Random seeds set at all levels (numpy, torch, sklearn). With these five, I can reproduce any experiment months later. The tradeoff is overhead: this adds 10-15 minutes of setup per project but saves hours of debugging when a model unexpectedly degrades and you need to trace back to what changed.

### Q2: How do you decide when to retrain a model in production?

**Sample Answer:** I use three triggers: (1) Scheduled retraining — weekly or monthly depending on data velocity. This is the simplest and works when data distribution is relatively stable. (2) Drift-based — I monitor feature distributions and prediction distributions using PSI. When PSI exceeds 0.2 for any key feature, I trigger retraining. (3) Performance-based — when delayed labels arrive and actual performance drops below a threshold, retrain immediately. The tradeoff between scheduled and triggered retraining is predictability vs responsiveness. Scheduled retraining is easier to operationalize and debug, but may retrain unnecessarily or too late. I typically combine scheduled (as a safety net) with drift-triggered (for responsiveness).

### Q3: What's your approach to managing the transition from notebook experimentation to production ML?

**Sample Answer:** I use a graduated approach. Phase 1 (Exploration): Notebooks are fine, but even here I use MLflow to track experiments so nothing is lost. Phase 2 (Consolidation): Once a direction is promising, I refactor notebook code into modular Python scripts with Hydra configs. The key step is extracting data loading, feature engineering, training, and evaluation into separate functions with clear interfaces. Phase 3 (Productionization): Add DVC for data versioning, Great Expectations for data validation, pytest for quality gates, and CI/CD for automated training and deployment. The common mistake is trying to jump from notebooks directly to full production — the intermediate consolidation step is where most of the value comes from, and it's often sufficient for teams that don't need real-time inference.

### Q4: How do you handle feature stores in an MLOps pipeline?

**Sample Answer:** A feature store serves two purposes: consistency (same features in training and serving) and reuse (features computed once, used by many models). For small teams, I start simple — a shared feature engineering script that writes to parquet files, versioned by DVC. This avoids the operational complexity of a dedicated feature store while ensuring training/serving consistency. For larger organizations with multiple models sharing features, I use a proper feature store (Feast or Tecton) that provides: a catalog of available features, point-in-time correct joins for training data (preventing data leakage), and a low-latency serving layer for online features. The tradeoff is significant operational complexity — a feature store is infrastructure that needs its own monitoring, scaling, and maintenance. I only recommend it when feature reuse across 3+ models justifies the overhead.
