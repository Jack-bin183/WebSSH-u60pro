type WorkerStartMessage = {
  type: 'start';
  id: number;
  url: string;
  token: string;
  loop: boolean;
};

type WorkerStopMessage = {
  type: 'stop';
};

let controller: AbortController | null = null;
let stopped = false;

function post(data: Record<string, unknown>) {
  self.postMessage(data);
}

async function runSpeedtest(message: WorkerStartMessage) {
  stopped = false;
  controller = new AbortController();
  let rounds = 0;

  try {
    do {
      rounds += 1;
      const requestUrl = `/api/speedtest?url=${encodeURIComponent(message.url)}&worker=${message.id}&round=${rounds}&t=${Date.now()}`;
      const res = await fetch(requestUrl, {
        signal: controller.signal,
        cache: 'no-store',
        headers: message.token ? { Authorization: message.token } : {},
      });

      if (!res.ok) {
        let detail = '';
        try {
          const body = await res.json();
          detail = body?.msg ? `: ${body.msg}` : '';
        } catch {
          detail = '';
        }
        throw new Error(`请求失败: ${res.status}${detail}`);
      }

      const contentLength = Number(res.headers.get('content-length') || 0);
      if (Number.isFinite(contentLength) && contentLength > 0) {
        post({ type: 'length', id: message.id, bytes: contentLength, round: rounds });
      }

      const reader = res.body?.getReader();
      if (!reader) throw new Error('浏览器不支持流式读取');

      while (!stopped) {
        const { done, value } = await reader.read();
        if (done) break;
        if (value?.length) post({ type: 'progress', id: message.id, bytes: value.length, round: rounds });
      }

      try {
        reader.releaseLock();
      } catch {
        // ignore
      }
    } while (message.loop && !stopped);

    post({ type: stopped ? 'stopped' : 'done', id: message.id, rounds });
  } catch (err: any) {
    if (err?.name === 'AbortError' || stopped) {
      post({ type: 'stopped', id: message.id, rounds });
    } else {
      post({ type: 'error', id: message.id, message: err?.message || String(err), rounds });
    }
  } finally {
    controller = null;
  }
}

self.onmessage = (event: MessageEvent<WorkerStartMessage | WorkerStopMessage>) => {
  if (event.data.type === 'stop') {
    stopped = true;
    controller?.abort();
    return;
  }
  if (event.data.type === 'start') {
    void runSpeedtest(event.data);
  }
};
