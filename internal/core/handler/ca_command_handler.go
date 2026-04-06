package handler

import (
	"fmt"
	"strings"
	"time"

	"dx/internal/cli/output"
	"dx/internal/core"
	"dx/internal/ports"
)

type CACommandHandler struct {
	configRepository       ports.ConfigRepository
	certificateAuthority   ports.CertificateAuthority
	certificateProvisioner *core.CertificateProvisioner
	terminalInput          ports.TerminalInput
	environmentEnsurer     core.EnvironmentEnsurer
}

func ProvideCACommandHandler(
	configRepository ports.ConfigRepository,
	certificateAuthority ports.CertificateAuthority,
	certificateProvisioner *core.CertificateProvisioner,
	terminalInput ports.TerminalInput,
	environmentEnsurer core.EnvironmentEnsurer,
) CACommandHandler {
	return CACommandHandler{
		configRepository:       configRepository,
		certificateAuthority:   certificateAuthority,
		certificateProvisioner: certificateProvisioner,
		terminalInput:          terminalInput,
		environmentEnsurer:     environmentEnsurer,
	}
}

// HandlePrint prints the CA certificate PEM to stdout.
func (h *CACommandHandler) HandlePrint() error {
	contextName, err := h.configRepository.LoadCurrentContextName()
	if err != nil {
		return err
	}

	certPEM, err := h.certificateAuthority.GetCACertificatePEM(contextName)
	if err != nil {
		return fmt.Errorf(
			"no certificate authority exists for context '%s'; run 'dx install' or 'dx ca recreate' to create one",
			contextName,
		)
	}

	fmt.Print(string(certPEM))
	return nil
}

// HandleReissue re-issues all certificates using the existing CA.
func (h *CACommandHandler) HandleReissue() error {
	if err := h.environmentEnsurer.EnsureExpectedClusterIsSelected(); err != nil {
		return err
	}

	configContext, err := h.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return err
	}

	_, caErr := h.certificateAuthority.GetCACertificatePEM(configContext.Name)
	if caErr != nil {
		return fmt.Errorf(
			"no certificate authority exists for context '%s'; run 'dx install' or 'dx ca recreate' to create one",
			configContext.Name,
		)
	}

	services := core.CollectAllCertificates(configContext.Services, configContext)

	output.PrintHeader("Re-issuing certificates")
	output.PrintNewline()

	secrets, err := h.certificateProvisioner.ReissueCertificates(services, configContext.Name)
	if err != nil {
		return err
	}

	for _, name := range secrets {
		output.PrintBullet(name)
	}
	output.PrintNewline()
	output.PrintSuccess(
		fmt.Sprintf(
			"Re-issued %d %s",
			len(secrets), output.Plural(len(secrets), "certificate", "certificates"),
		),
	)
	output.PrintInfo("Run 'dx install' to apply the new certificates")

	return nil
}

// HandleRecreate deletes the CA and recreates it, then re-issues all certificates.
func (h *CACommandHandler) HandleRecreate(skipConfirmation bool) error {
	if err := h.environmentEnsurer.EnsureExpectedClusterIsSelected(); err != nil {
		return err
	}

	configContext, err := h.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return err
	}
	contextName := configContext.Name

	// Check if a CA exists to determine if we need confirmation
	_, caErr := h.certificateAuthority.GetCACertificatePEM(contextName)
	caExists := caErr == nil

	if caExists && !skipConfirmation {
		if !h.terminalInput.IsTerminal() {
			return fmt.Errorf("recreating the CA requires confirmation. Use --yes to skip in non-interactive mode")
		}

		output.PrintWarning(
			fmt.Sprintf(
				"This will delete the existing CA and all certificates for context '%s'.",
				contextName,
			),
		)
		services := core.CollectAllCertificates(configContext.Services, configContext)
		for _, svc := range services {
			for _, cert := range svc.Certificates {
				output.PrintWarningDetail(cert.K8sSecret.Name)
			}
		}
		output.PrintWarningSecondary("After recreating, you must re-trust the new CA certificate.")
		output.PrintWarningNewline()

		response, err := h.terminalInput.ReadLine("Continue? [y/N] ")
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			output.PrintInfo("Recreate cancelled")
			return nil
		}
		output.PrintNewline()
	}

	if caExists {
		output.PrintHeader("Recreating certificate authority")
	} else {
		output.PrintHeader("Creating certificate authority")
	}
	output.PrintNewline()

	// Delete existing CA and passphrase
	if caExists {
		output.PrintStep("Removing existing CA")
		if err := h.certificateAuthority.DeleteCA(contextName); err != nil {
			return fmt.Errorf("failed to remove existing CA: %w", err)
		}
		if err := h.certificateProvisioner.DeletePassphrase(contextName); err != nil {
			return fmt.Errorf("failed to remove CA passphrase: %w", err)
		}
	}

	// Re-issue all certificates (this will create a new CA via LoadOrCreateCA)
	output.PrintStep("Creating new CA")

	services := core.CollectAllCertificates(configContext.Services, configContext)
	secrets, err := h.certificateProvisioner.ReissueCertificates(services, contextName)
	if err != nil {
		return err
	}

	for _, name := range secrets {
		output.PrintBullet(name)
	}
	output.PrintNewline()
	if caExists {
		output.PrintSuccess(
			fmt.Sprintf(
				"Recreated CA and re-issued %d %s",
				len(secrets), output.Plural(len(secrets), "certificate", "certificates"),
			),
		)
	} else {
		output.PrintSuccess(
			fmt.Sprintf(
				"Created CA and issued %d %s",
				len(secrets), output.Plural(len(secrets), "certificate", "certificates"),
			),
		)
	}

	output.PrintInfo("Run 'dx install' to apply the new certificates")
	output.PrintInfo("Run 'dx ca print' to retrieve the new CA certificate for your trust store")

	return nil
}

// HandleStatus shows the status of the certificate authority and all certificates.
func (h *CACommandHandler) HandleStatus() error {
	if err := h.environmentEnsurer.EnsureExpectedClusterIsSelected(); err != nil {
		return err
	}

	contextName, err := h.configRepository.LoadCurrentContextName()
	if err != nil {
		return err
	}

	expiry, err := h.certificateAuthority.GetCACertificateExpiry(contextName)
	if err != nil {
		output.PrintInfo("No CA exists for this context; one will be created automatically on 'dx install'")
		return nil
	}

	output.PrintHeader("Certificate Authority")
	output.PrintNewline()

	now := time.Now()
	daysRemaining := int(expiry.Sub(now).Hours() / 24)
	status := "valid"
	statusStyled := output.Success(status)
	if daysRemaining <= 0 {
		status = "expired"
		statusStyled = output.Error(status)
	} else if daysRemaining <= 30 {
		status = "expiring soon"
		statusStyled = output.Warning(status)
	}

	output.PrintField("Context:", contextName)
	output.PrintField("Status:", statusStyled)
	if daysRemaining <= 0 {
		output.PrintField("Expired:", fmt.Sprintf(
			"%s (%d %s ago)",
			expiry.Format("2006-01-02"),
			-daysRemaining,
			output.Plural(-daysRemaining, "day", "days"),
		))
	} else {
		output.PrintField("Expires:", fmt.Sprintf(
			"%s (%d %s remaining)",
			expiry.Format("2006-01-02"),
			daysRemaining,
			output.Plural(daysRemaining, "day", "days"),
		))
	}
	output.PrintField("Cert path:", fmt.Sprintf("~/.dx/%s/ca/ca.crt", contextName))

	// List certificate statuses
	configContext, err := h.configRepository.LoadCurrentConfigurationContext()
	if err != nil {
		return err
	}

	services := core.CollectAllCertificates(configContext.Services, configContext)
	statuses, err := h.certificateProvisioner.GetCertificateStatuses(services)
	if err != nil {
		return err
	}

	if len(statuses) > 0 {
		output.PrintNewline()
		output.PrintHeader("Certificates")

		currentService := ""
		for _, s := range statuses {
			if s.ServiceName != currentService {
				currentService = s.ServiceName
				output.PrintNewline()
				if currentService == "dev-proxy" {
					output.PrintLabel(output.Bold(currentService) + " " + output.Dim("(internal)"))
				} else {
					output.PrintLabel(output.Bold(currentService))
				}
			}
			output.PrintBulletField(s.SecretName, formatCertStatus(s))
			output.PrintSubfield("Type:", output.Dim(string(s.CertType)))
			output.PrintSubfield("SANs:", output.Dim(strings.Join(s.DNSNames, ", ")))
		}
		output.PrintNewline()
	}

	return nil
}

func formatCertStatus(s core.CertificateStatus) string {
	if !s.Found {
		return output.Dim("not provisioned")
	}
	if s.DaysRemaining <= 0 {
		return output.Error(
			fmt.Sprintf(
				"expired (%d %s ago)",
				-s.DaysRemaining, output.Plural(-s.DaysRemaining, "day", "days"),
			),
		)
	}
	if s.DaysRemaining <= 14 {
		return output.Warning(
			fmt.Sprintf(
				"expiring soon (%d %s remaining)",
				s.DaysRemaining, output.Plural(s.DaysRemaining, "day", "days"),
			),
		)
	}
	return output.Success(
		fmt.Sprintf(
			"valid (%d %s remaining)",
			s.DaysRemaining, output.Plural(s.DaysRemaining, "day", "days"),
		),
	)
}
