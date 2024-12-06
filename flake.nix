{
  description = "Development shell for building and developing eBPF programs";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in {
      devShells.${system}.default = pkgs.mkShell {
        # Disable certain hardening features, otherwise we can't build the eBPF programs
        hardeningDisable = [
          "stackprotector"
          "zerocallusedregs"
        ];

        # Tools needed for developing eBPF programs
        buildInputs = [
          pkgs.clang
          pkgs.llvmPackages_19.clangUseLLVM
          pkgs.llvm_19
          pkgs.libbpf
        ];

        nativeBuildInputs = [
          pkgs.pkg-config
        ];

        shellHook = ''
          echo "Entering eBPF development shell..."
          clang --version
          echo "You can now compile eBPF programs and use bpftools to inspect them."
        '';
      };
    };
}
