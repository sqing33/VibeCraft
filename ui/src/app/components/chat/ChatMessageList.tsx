import { ArrowDown } from "lucide-react";
import {
  forwardRef,
  type HTMLAttributes,
  type ReactNode,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Virtuoso, type VirtuosoHandle } from "react-virtuoso";

import clsx from "clsx";

import type { ChatMessage } from "@/lib/daemon";

type TranscriptItem =
  | {
      kind: "message";
      message: ChatMessage;
    }
  | {
      kind: "pending";
    };

const BASE_ITEM_INDEX = 100_000;

const Scroller = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  function Scroller({ className, ...props }, ref) {
    return (
      <div
        {...props}
        ref={ref}
        className={clsx(
          "min-h-0 flex-1 overflow-x-hidden overflow-y-auto px-4 py-6 md:px-8",
          className,
        )}
      />
    );
  },
);

const List = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  function List({ className, ...props }, ref) {
    return (
      <div
        {...props}
        ref={ref}
        className={clsx(
          "mx-auto flex w-full max-w-[880px] min-w-0 flex-col gap-5",
          className,
        )}
      />
    );
  },
);

export type ChatMessageListProps = {
  sessionId: string | null;
  messages: ChatMessage[];
  pending?: boolean;
  pendingAutoscrollKey?: number;
  onStartReached?: () => void;
  renderMessage: (message: ChatMessage) => ReactNode;
  renderPending?: () => ReactNode;
};

export function ChatMessageList({
  sessionId,
  messages,
  pending = false,
  pendingAutoscrollKey,
  onStartReached,
  renderMessage,
  renderPending,
}: ChatMessageListProps) {
  const virtuosoRef = useRef<VirtuosoHandle>(null);
  const didInitScrollRef = useRef(false);
  const prevFirstMessageIdRef = useRef<string | null>(null);
  const prevMessageCountRef = useRef(0);
  const [firstItemIndex, setFirstItemIndex] = useState(BASE_ITEM_INDEX);
  const [atBottom, setAtBottom] = useState(true);

  const data = useMemo(() => {
    const items: TranscriptItem[] = messages.map((message) => ({
      kind: "message",
      message,
    }));
    if (pending) items.push({ kind: "pending" });
    return items;
  }, [messages, pending]);

  useEffect(() => {
    setFirstItemIndex(BASE_ITEM_INDEX);
    didInitScrollRef.current = false;
    prevFirstMessageIdRef.current = null;
    prevMessageCountRef.current = 0;
    setAtBottom(true);
  }, [sessionId]);

  useEffect(() => {
    if (!sessionId) return;
    if (didInitScrollRef.current) return;
    if (messages.length === 0 && !pending) return;
    didInitScrollRef.current = true;
    requestAnimationFrame(() => {
      virtuosoRef.current?.scrollToIndex({
        index: "LAST",
        align: "end",
        behavior: "auto",
      });
    });
  }, [messages.length, pending, sessionId]);

  useLayoutEffect(() => {
    const prevFirst = prevFirstMessageIdRef.current;
    const prevCount = prevMessageCountRef.current;
    const nextFirst = messages[0]?.message_id ?? null;

    if (
      sessionId &&
      prevFirst &&
      nextFirst &&
      messages.length > prevCount &&
      prevFirst !== nextFirst
    ) {
      const shift = messages.findIndex(
        (message) => message.message_id === prevFirst,
      );
      if (shift > 0) {
        setFirstItemIndex((value) => value - shift);
      }
    }

    prevFirstMessageIdRef.current = nextFirst;
    prevMessageCountRef.current = messages.length;
  }, [messages, sessionId]);

  useEffect(() => {
    if (!pending) return;
    if (!atBottom) return;
    requestAnimationFrame(() => {
      virtuosoRef.current?.scrollToIndex({
        index: "LAST",
        align: "end",
        behavior: "auto",
      });
    });
  }, [atBottom, pending, pendingAutoscrollKey]);

  const handleScrollToBottom = () => {
    virtuosoRef.current?.scrollToIndex({
      index: "LAST",
      align: "end",
      behavior: "smooth",
    });
  };

  return (
    <div className="relative min-h-0 min-w-0 flex-1 overflow-x-hidden">
      <Virtuoso<TranscriptItem>
        ref={virtuosoRef}
        className="min-w-0"
        data={data}
        firstItemIndex={firstItemIndex}
        atBottomThreshold={100}
        increaseViewportBy={200}
        startReached={onStartReached}
        atBottomStateChange={setAtBottom}
        followOutput={atBottom ? "auto" : false}
        computeItemKey={(_index, item) =>
          item.kind === "message" ? item.message.message_id : "pending"
        }
        itemContent={(_index, item) => {
          if (item.kind === "pending") {
            return (
              <div className="mx-auto w-full max-w-[880px]">
                {renderPending ? renderPending() : null}
              </div>
            );
          }
          return renderMessage(item.message);
        }}
        components={{ Scroller, List }}
      />

      {!atBottom ? (
        <div className="pointer-events-none absolute bottom-4 left-0 right-0 flex justify-center">
          <button
            type="button"
            className="pointer-events-auto flex items-center gap-2 rounded-full border border-default-200/70 bg-background/90 px-3 py-2 text-xs text-muted-foreground shadow-sm backdrop-blur transition hover:bg-background hover:text-foreground"
            onClick={handleScrollToBottom}
            title="回到底部"
          >
            <ArrowDown className="h-4 w-4" />
            回到底部
          </button>
        </div>
      ) : null}
    </div>
  );
}
