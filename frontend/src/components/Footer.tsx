"use client";

const Footer = () => {
  return (
    <footer className="fixed bottom-0 w-full bg-background border-t">
      <div className="container mx-auto py-4 text-center text-sm text-muted-foreground">
        © {new Date().getFullYear()} Sainik. All rights reserved.
      </div>
    </footer>
  );
};

export default Footer;
