"use client";



import { useEffect, useRef, useState } from "react";

import { usePathname, useRouter } from "next/navigation";

import { runtime } from "@sparkops/web-sdk";

import { BffError } from "@sparkops/contracts";

import type { MessageVM } from "@sparkops/contracts/runtime";



interface ChatPanelProps {

  agentId: string;

  workspaceId: string;

  initialConversationId?: string;

  initialMessages?: MessageVM[];

  canChat?: boolean;

}



type UiMessage = {

  id: string;

  role: "user" | "assistant";

  content: string;

};



export function ChatPanel({

  agentId,

  workspaceId,

  initialConversationId,

  initialMessages = [],

  canChat = true,

}: ChatPanelProps) {

  const router = useRouter();

  const pathname = usePathname();

  const [conversationId, setConversationId] = useState(initialConversationId);

  const [messages, setMessages] = useState<UiMessage[]>(

    initialMessages.map((m) => ({

      id: m.id,

      role: m.role === "assistant" ? "assistant" : "user",

      content: m.content,

    })),

  );

  const [input, setInput] = useState("");

  const [streaming, setStreaming] = useState(false);

  const [error, setError] = useState<string | null>(null);

  const bottomRef = useRef<HTMLDivElement>(null);



  useEffect(() => {

    bottomRef.current?.scrollIntoView({ behavior: "smooth" });

  }, [messages, streaming]);



  async function onSubmit(e: React.FormEvent) {

    e.preventDefault();

    if (!canChat) {

      return;

    }

    const text = input.trim();

    if (!text || streaming) {

      return;

    }



    setInput("");

    setError(null);

    setStreaming(true);



    const userMsgId = `user-${Date.now()}`;

    const assistantMsgId = `assistant-${Date.now()}`;

    setMessages((prev) => [

      ...prev,

      { id: userMsgId, role: "user", content: text },

      { id: assistantMsgId, role: "assistant", content: "" },

    ]);



    try {

      const start = await runtime.startChat(

        agentId,

        { message: text, conversation_id: conversationId },

        workspaceId,

      );

      setConversationId(start.conversation_id);

      router.replace(`${pathname}?conversationId=${start.conversation_id}`, { scroll: false });

      router.refresh();



      runtime.streamRun(start.run_id, {

        onDelta: (delta) => {

          setMessages((prev) =>

            prev.map((m) =>

              m.id === assistantMsgId ? { ...m, content: m.content + delta } : m,

            ),

          );

        },

        onDone: () => setStreaming(false),

        onError: (err) => {

          setError(err.message);

          setStreaming(false);

        },

      });

    } catch (err) {

      setStreaming(false);

      setError(err instanceof BffError ? err.message : "发送失败");

    }

  }



  return (

    <div className="chat-shell">

      <div className="chat-messages space-y-4">

        {messages.length === 0 ? (

          <p className="text-muted text-center py-10">发送消息开始对话</p>

        ) : null}

        {messages.map((msg) => (

          <div

            key={msg.id}

            className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}

          >

            <div

              className={

                msg.role === "user" ? "msg-bubble-user whitespace-pre-wrap" : "msg-bubble-assistant whitespace-pre-wrap"

              }

            >

              {msg.content || (streaming && msg.role === "assistant" ? "..." : "")}

            </div>

          </div>

        ))}

        <div ref={bottomRef} />

      </div>



      <form className="chat-composer space-y-2" onSubmit={onSubmit}>

        {error ? <p className="error">{error}</p> : null}

        <div className="flex gap-2">

          <input

            className="input flex-1"

            placeholder="输入消息..."

            value={input}

            onChange={(e) => setInput(e.target.value)}

            disabled={streaming || !canChat}

          />

          <button

            className="btn shrink-0"

            type="submit"

            disabled={streaming || !input.trim() || !canChat}

          >

            {streaming ? "生成中..." : "发送"}

          </button>

        </div>

      </form>

    </div>

  );

}

