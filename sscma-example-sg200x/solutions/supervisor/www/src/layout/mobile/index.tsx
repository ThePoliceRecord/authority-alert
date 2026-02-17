import React from "react";
import Header from "@/layout/mobile/header";

interface Props {
  children: React.ReactNode;
}

const MobileLayout: React.FC<Props> = ({ children }) => {
  return (
    <>
      <Header />
      <div 
        className="flex-1 overflow-y-auto"
        style={{
          backgroundImage: `linear-gradient(rgba(0, 0, 0, 0.6), rgba(0, 0, 0, 0.6)), url('/brand/blueback.jpeg')`,
          backgroundSize: 'cover',
          backgroundPosition: 'center',
          backgroundRepeat: 'no-repeat',
          backgroundAttachment: 'fixed',
        }}
      >
        {children}
      </div>
    </>
  );
};

export default MobileLayout;
