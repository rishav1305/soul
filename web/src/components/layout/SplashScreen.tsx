import { useState, useEffect } from 'react';

interface SplashScreenProps {
  ready: boolean;
}

export default function SplashScreen({ ready }: SplashScreenProps) {
  const [visible, setVisible] = useState(true);

  useEffect(() => {
    if (ready) {
      // Start fade-out, then unmount
      const timer = setTimeout(() => setVisible(false), 600);
      return () => clearTimeout(timer);
    }
  }, [ready]);

  if (!visible) return null;

  return (
    <div
      className={`fixed inset-0 z-50 bg-deep noise flex flex-col items-center justify-center ${
        ready ? 'animate-fade-out' : ''
      }`}
    >
      {/* Glow ring */}
      <div className="relative">
        <div className="absolute inset-0 -m-16 bg-soul/15 rounded-full blur-3xl animate-soul-pulse" />
        <div className="relative text-9xl text-soul animate-float">&#9670;</div>
      </div>

      {/* Title */}
      <p className="font-display text-2xl tracking-[0.3em] text-fg-secondary mt-10 uppercase">
        Soul
      </p>

      {/* Loading dots */}
      <div className="flex gap-2 mt-8">
        {[0, 1, 2].map((i) => (
          <span
            key={i}
            className="w-1.5 h-1.5 rounded-full bg-soul animate-soul-pulse"
            style={{ animationDelay: `${i * 200}ms` }}
          />
        ))}
      </div>
    </div>
  );
}
