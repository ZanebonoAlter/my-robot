"""LLM 客户端:对接本地 llama.cpp 的 OpenAI 兼容接口。

关键点:
- Qwen3 默认开 thinking 模式,思考进 reasoning_content,正式回答进 content。
  所以每次调用要给足 max_tokens(否则光在想就 finish=length)。
- 不依赖 SDK,纯 requests,跟 Syntopica 后端 airouter 的做法一致(便于将来移植到 Go)。
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field

import requests

BASE_URL = "http://localhost:8080/v1"
MODEL = "Qwen3.5-9B"
DEFAULT_MAX_TOKENS = 2000  # 关闭 thinking 后,2000 足够结构化输出


@dataclass
class LLMResponse:
    content: str
    reasoning: str
    finish_reason: str
    usage: dict


@dataclass
class Message:
    role: str
    content: str

    def to_dict(self) -> dict:
        return {"role": self.role, "content": self.content}


def chat(
    messages: list[Message],
    *,
    max_tokens: int = DEFAULT_MAX_TOKENS,
    temperature: float = 0.4,
    json_mode: bool = False,
) -> LLMResponse:
    """调用 /v1/chat/completions,返回结构化响应。"""
    payload = {
        "model": MODEL,
        "messages": [m.to_dict() for m in messages],
        "max_tokens": max_tokens,
        "temperature": temperature,
        # Qwen3 默认开 thinking,会烧大量 token 且 agent loop 场景不需要长考。
        # 通过 chat_template_kwargs 彻底关闭,让输出直接出结构化内容。
        "chat_template_kwargs": {"enable_thinking": False},
    }
    if json_mode:
        payload["response_format"] = {"type": "json_object"}

    resp = requests.post(f"{BASE_URL}/chat/completions", json=payload, timeout=300)
    resp.raise_for_status()
    data = resp.json()
    msg = data["choices"][0]["message"]
    return LLMResponse(
        content=msg.get("content", "") or "",
        reasoning=msg.get("reasoning_content", "") or "",
        finish_reason=data["choices"][0]["finish_reason"],
        usage=data.get("usage", {}),
    )


def parse_json_response(text: str) -> dict | None:
    """从 LLM 输出里抠出 JSON。

    Qwen3 有时会带 ```json 代码块或前后说明文字,这里做容错。
    """
    text = text.strip()
    # 去掉 markdown 代码块
    if text.startswith("```"):
        text = text.split("\n", 1)[1] if "\n" in text else text[3:]
        if text.endswith("```"):
            text = text[:-3]
        text = text.strip()
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        # 尝试找第一个 { 到最后一个 }
        start = text.find("{")
        end = text.rfind("}")
        if start != -1 and end != -1 and end > start:
            try:
                return json.loads(text[start : end + 1])
            except json.JSONDecodeError:
                pass
    return None
