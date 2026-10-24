"""Configuration for the Python side (MCP server, dbinit, agent)."""

from __future__ import annotations

from functools import lru_cache
from typing import Final, Literal

from pydantic import Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict

DEFAULT_MODELS: Final = "anthropic/claude-sonnet-4.5,google/gemini-2.5-flash"

DEFAULT_BASE_URL: Final = "https://openrouter.ai/api/v1"

LogLevel = Literal["debug", "info", "warning", "error"]


class Settings(BaseSettings):
    model_config = SettingsConfigDict(extra="ignore", case_sensitive=False)

    app_env: str = Field(default="local", alias="APP_ENV")
    app_timezone: str = Field(default="Europe/Moscow", alias="APP_TIMEZONE")
    nlq_today: str | None = Field(default=None, alias="NLQ_TODAY")
    log_level: LogLevel = Field(default="info", alias="LOG_LEVEL")

    nlq_actor_public_key: str = Field(default="", alias="NLQ_ACTOR_PUBLIC_KEY")

    # Любой OpenAI-совместимый API: LLM_BASE_URL + LLM_API_KEY. Имена OPENROUTER_* оставлены для совместимости.
    llm_api_key: str = Field(default="", alias="LLM_API_KEY")
    llm_base_url: str = Field(default="", alias="LLM_BASE_URL")
    openrouter_api_key: str = Field(default="", alias="OPENROUTER_API_KEY")
    openrouter_base_url: str = Field(default="", alias="OPENROUTER_BASE_URL")
    # Для новых моделей OpenAI нужно max_completion_tokens.
    llm_max_tokens_field: Literal["max_tokens", "max_completion_tokens"] = Field(
        default="max_tokens", alias="LLM_MAX_TOKENS_FIELD"
    )
    nlq_models: str = Field(default=DEFAULT_MODELS, alias="NLQ_MODELS")
    nlq_llm: Literal["llm", "rules", "fake"] = Field(default="llm", alias="NLQ_LLM")
    nlq_rules_fallback: bool = Field(default=True, alias="NLQ_RULES_FALLBACK")
    nlq_mcp_url: str = Field(default="http://nlq-mcp:8900/mcp", alias="NLQ_MCP_URL")
    nlq_max_llm_calls: int = Field(default=4, ge=1, le=8, alias="NLQ_MAX_LLM_CALLS")
    nlq_max_tool_calls_per_step: int = Field(
        default=3, ge=1, le=6, alias="NLQ_MAX_TOOL_CALLS_PER_STEP"
    )
    nlq_step_timeout: float = Field(default=10.0, gt=0, alias="NLQ_STEP_TIMEOUT")
    nlq_tool_timeout: float = Field(default=5.0, gt=0, alias="NLQ_TOOL_TIMEOUT")
    nlq_total_timeout: float = Field(default=22.0, gt=0, alias="NLQ_TOTAL_TIMEOUT")
    nlq_max_tokens: int = Field(default=700, ge=1, alias="NLQ_MAX_TOKENS")
    nlq_public_url: str = Field(default="https://localhost", alias="NLQ_PUBLIC_URL")
    nlq_fake_script: str | None = Field(default=None, alias="NLQ_FAKE_SCRIPT")
    agent_host: str = Field(default="0.0.0.0", alias="NLQ_AGENT_HOST")
    agent_port: int = Field(default=8090, alias="NLQ_AGENT_PORT")

    nlq_db_dsn: str = Field(default="", alias="NLQ_DB_DSN")
    nlq_user_scope: Literal["team", "own"] = Field(default="team", alias="NLQ_USER_SCOPE")
    nlq_max_rows: int = Field(default=20, ge=1, le=20, alias="NLQ_MAX_ROWS")
    nlq_dev_schema: str | None = Field(default=None, alias="NLQ_DEV_SCHEMA")
    mcp_host: str = Field(default="0.0.0.0", alias="NLQ_MCP_HOST")
    mcp_port: int = Field(default=8900, alias="NLQ_MCP_PORT")

    admin_dsn: str = Field(default="", alias="ADMIN_DSN")
    postgres_user: str = Field(default="", alias="POSTGRES_USER")
    postgres_password: str = Field(default="", alias="POSTGRES_PASSWORD")
    postgres_db: str = Field(default="", alias="POSTGRES_DB")
    postgres_host: str = Field(default="postgres", alias="POSTGRES_HOST")
    postgres_port: int = Field(default=5432, alias="POSTGRES_PORT")
    nlq_db_password: str = Field(default="", alias="NLQ_DB_PASSWORD")

    @property
    def api_key(self) -> str:
        return self.llm_api_key or self.openrouter_api_key

    @property
    def base_url(self) -> str:
        url = self.llm_base_url or self.openrouter_base_url or DEFAULT_BASE_URL
        return url.rstrip("/")

    @property
    def is_openrouter(self) -> bool:
        """OpenRouter понимает дополнительные поля запроса, остальные API их не ждут."""
        return "openrouter.ai" in self.base_url

    @property
    def is_production(self) -> bool:
        return self.app_env.lower() == "production"

    @property
    def dev_schema(self) -> str | None:
        """Dev fixtures schema, honoured only outside production."""
        if self.is_production:
            return None
        return self.nlq_dev_schema or None

    @property
    def today_override(self) -> str | None:
        """NLQ_TODAY is honoured only outside production."""
        if self.is_production:
            return None
        return self.nlq_today or None

    @property
    def models(self) -> list[str]:
        """``NLQ_MODELS`` as an ordered [primary, backup, ...] list."""
        return [m.strip() for m in self.nlq_models.split(",") if m.strip()]

    def resolved_admin_dsn(self) -> str:
        """DSN for the superuser one-shot: ADMIN_DSN, else built from POSTGRES_*."""
        if self.admin_dsn:
            return self.admin_dsn
        if not (self.postgres_user and self.postgres_db):
            raise ValueError("ADMIN_DSN or POSTGRES_USER/POSTGRES_DB must be set")
        pw = f":{self.postgres_password}" if self.postgres_password else ""
        return (
            f"postgres://{self.postgres_user}{pw}@"
            f"{self.postgres_host}:{self.postgres_port}/{self.postgres_db}"
        )

    @field_validator("nlq_llm", mode="before")
    @classmethod
    def _norm_llm_mode(cls, v: object) -> object:
        if isinstance(v, str):
            v = v.strip().lower()
            if v in ("openrouter", "openai"):
                return "llm"
        return v

    @field_validator("nlq_user_scope", mode="before")
    @classmethod
    def _norm_scope(cls, v: object) -> object:
        return v.lower() if isinstance(v, str) else v


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    return Settings()
