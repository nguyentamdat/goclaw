import { Cpu } from "lucide-react";
import { cn } from "@/lib/utils";

const MODELS_DEV_LOGO_BASE = "https://models.dev/logos";

/**
 * Maps goclaw provider_type to models.dev provider ID for logo URLs.
 * See https://models.dev/api.json for available provider IDs.
 */
const PROVIDER_TYPE_TO_LOGO: Record<string, string> = {
  anthropic_native: "anthropic",
  chatgpt_oauth: "openai",
  openai_compat: "openai",
  gemini_native: "google",
  openrouter: "openrouter",
  groq: "groq",
  deepseek: "deepseek",
  mistral: "mistral",
  xai: "xai",
  minimax_native: "minimax",
  cohere: "cohere",
  perplexity: "perplexity",
  dashscope: "alibaba",
  bailian: "alibaba-coding-plan",
  ollama: "ollama-cloud",
  ollama_cloud: "ollama-cloud",
  claude_cli: "anthropic",
  acp: "anthropic",
  yescale: "yescale",
  zai: "zai",
  zai_coding: "zai-coding-plan",
  byteplus: "byteplus",
  byteplus_coding: "byteplus",
};

export function getProviderLogoUrl(providerType: string): string | null {
  const logoId = PROVIDER_TYPE_TO_LOGO[providerType];
  if (!logoId) return null;
  return `${MODELS_DEV_LOGO_BASE}/${logoId}.svg`;
}

interface ProviderLogoProps {
  providerType: string;
  className?: string;
  /** Size in pixels for the img, defaults to 16 */
  size?: number;
}

/**
 * Renders the provider logo from models.dev, falling back to a Cpu icon.
 */
export function ProviderLogo({ providerType, className, size = 16 }: ProviderLogoProps) {
  const logoUrl = getProviderLogoUrl(providerType);

  if (!logoUrl) {
    return <Cpu className={cn("h-4 w-4", className)} />;
  }

  return (
    <img
      src={logoUrl}
      alt=""
      width={size}
      height={size}
      className={cn("shrink-0 dark:invert", className)}
      loading="lazy"
      onError={(e) => {
        // Hide broken image, show nothing (parent already has bg)
        e.currentTarget.style.display = "none";
      }}
    />
  );
}
