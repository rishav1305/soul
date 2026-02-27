import { useEffect, useRef } from 'react';
import { useChat } from '../../hooks/useChat.ts';
import Message from './Message.tsx';
import InputBar from './InputBar.tsx';

export default function ChatView() {
  const { messages, sendMessage, isStreaming } = useChat();
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  return (
    <div className="flex flex-col h-full">
      <div ref={scrollRef} className="flex-1 overflow-y-auto px-4 py-6">
        <div className="max-w-3xl mx-auto space-y-4">
          {messages.length === 0 && (
            <div className="flex items-center justify-center h-full min-h-[200px]">
              <div className="text-center text-zinc-600">
                <div className="text-4xl mb-3">&#9670;</div>
                <p className="text-lg">How can I help you?</p>
              </div>
            </div>
          )}
          {messages.map((msg) => (
            <Message key={msg.id} message={msg} />
          ))}
          {isStreaming && (
            <div className="flex justify-start">
              <div className="text-zinc-500 text-sm px-4 py-2">
                Soul is thinking...
              </div>
            </div>
          )}
        </div>
      </div>
      <InputBar onSend={sendMessage} disabled={isStreaming} />
    </div>
  );
}
