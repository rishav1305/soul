import React, { useState, useEffect } from 'react';
import { Text } from 'ink';
import { theme } from '../theme.js';
const frames = ['\u25CF', '\u25CB'];
interface SpinnerProps { label: string; }
export function Spinner({ label }: SpinnerProps) {
  const [frame, setFrame] = useState(0);
  useEffect(() => { const timer = setInterval(() => setFrame((prev) => (prev + 1) % frames.length), 300); return () => clearInterval(timer); }, []);
  return <Text>{theme.brand(frames[frame])} {label}</Text>;
}
